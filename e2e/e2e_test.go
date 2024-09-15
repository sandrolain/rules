package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sandrolain/rules/api"
	"github.com/sandrolain/rules/app"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/proto"
)

func TestE2E(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "nats-data")
	if err != nil {
		t.Fatalf("Errore nella creazione del temporaneo directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	req := testcontainers.ContainerRequest{
		Image:        "nats:latest",
		ExposedPorts: []string{"4222/tcp"},
		Cmd:          []string{"-js", "-sd", "/tmp/nats"},
		WaitingFor:   wait.ForListeningPort("4222/tcp").WithStartupTimeout(30 * time.Second),
		Mounts: []testcontainers.ContainerMount{
			{
				Target: "/tmp/nats",
				Source: testcontainers.GenericTmpfsMountSource{},
			},
		},
	}

	natsContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		t.Fatalf("Error starting NATS container: %v", err)
	}

	// Wait for the container to be actually ready
	err = natsContainer.Start(ctx)
	if err != nil {
		t.Fatalf("Error starting NATS container: %v", err)
	}
	defer natsContainer.Terminate(ctx)

	// Ottieni l'indirizzo del container NATS
	host, err := natsContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Errore nell'ottenere l'host del container: %v", err)
	}
	port, err := natsContainer.MappedPort(ctx, "4222")
	if err != nil {
		t.Fatalf("Errore nell'ottenere la porta mappata: %v", err)
	}

	natsURL := fmt.Sprintf("nats://%s:%s", host, port.Port())

	// Configurazione dell'applicazione
	cfg := &app.Config{
		NatsURL:           natsURL,
		NatsInputSubject:  "rules.engine.input",
		NatsOutputSubject: "rules.engine.output",
		NatsInputStream:   "RULES_INPUT",
		NatsOutputStream:  "RULES_OUTPUT",
		LogLevel:          "info",
	}

	// Avvio dell'applicazione
	application, err := app.NewApp(cfg)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		if err := application.Run(); err != nil {
			t.Error(err)
		}
	}()

	// Attendere che l'applicazione sia pronta
	time.Sleep(2 * time.Second)

	// Connessione NATS per i test
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}

	// Creazione di un consumer durevole per lo stream di output
	_, err = js.AddConsumer(cfg.NatsOutputStream, &nats.ConsumerConfig{
		Durable:   "test-consumer",
		AckPolicy: nats.AckExplicitPolicy,
	})
	assert.NoError(t, err)

	// Test: Aggiunta di una policy
	t.Run("AddPolicy", func(t *testing.T) {
		policy := &api.Policy{
			Id:         "test_policy",
			Name:       "Test Policy",
			Expression: "true",
			Rules: []*api.Rule{
				{
					Name:       "Test Rule 1",
					Expression: "Result(input.value, false)",
				},
				{
					Name:       "Test Rule 2",
					Expression: "Result(int(input.value) * 2, int(input.value) > 30)",
				},
				{
					Name:       "Test Rule 3",
					Expression: "Result(100, false)", // Questa regola non dovrebbe essere eseguita se la regola 2 si ferma
				},
			},
			Thresholds: []*api.Threshold{
				{
					Id:    "low",
					Value: 5,
				},
				{
					Id:    "medium",
					Value: 15,
				},
				{
					Id:    "high",
					Value: 50,
				},
			},
		}

		req := &api.SetPolicyRequest{Policy: policy}
		reqData, err := proto.Marshal(req)
		assert.NoError(t, err)

		err = nc.Publish(api.SetPolicy, reqData)
		assert.NoError(t, err)

		// Attendere che la policy sia stata aggiunta
		time.Sleep(1 * time.Second)

		// Ottenere la policy appena aggiunta
		getReq := &api.GetPolicyRequest{Id: "test_policy"}
		getReqData, _ := proto.Marshal(getReq)

		msg, err := nc.Request(api.GetPolicy, getReqData, 5*time.Second)
		assert.NoError(t, err)

		var getResp api.GetPolicyResponse
		err = proto.Unmarshal(msg.Data, &getResp)
		assert.NoError(t, err)

		// Confrontare la policy impostata con quella caricata
		assert.Equal(t, policy.Id, getResp.Policy.Id)
		assert.Equal(t, policy.Name, getResp.Policy.Name)
		assert.Equal(t, policy.Expression, getResp.Policy.Expression)
		assert.Equal(t, len(policy.Rules), len(getResp.Policy.Rules))
		assert.Equal(t, len(policy.Thresholds), len(getResp.Policy.Thresholds))

		// Confrontare le regole
		for i, rule := range policy.Rules {
			assert.Equal(t, rule.Name, getResp.Policy.Rules[i].Name)
			assert.Equal(t, rule.Expression, getResp.Policy.Rules[i].Expression)
		}

		// Confrontare le soglie
		for i, threshold := range policy.Thresholds {
			assert.Equal(t, threshold.Id, getResp.Policy.Thresholds[i].Id)
			assert.Equal(t, threshold.Value, getResp.Policy.Thresholds[i].Value)
		}
	})

	// Test: Valutazione di una policy senza stop
	t.Run("EvaluatePolicyWithoutStop", func(t *testing.T) {
		sub, err := js.PullSubscribe(cfg.NatsOutputSubject, "test-consumer")
		assert.NoError(t, err)

		input := map[string]interface{}{
			"value": 20,
		}
		inputData, _ := json.Marshal(input)

		_, err = js.Publish(cfg.NatsInputSubject, inputData)
		assert.NoError(t, err)

		msgs, err := sub.Fetch(1, nats.MaxWait(5*time.Second))
		assert.NoError(t, err)
		assert.Len(t, msgs, 1)

		var policyResults api.PolicyResults
		err = proto.Unmarshal(msgs[0].Data, &policyResults)
		assert.NoError(t, err)

		results := policyResults.Results

		assert.Len(t, results, 1)
		assert.Equal(t, "test_policy", results[0].PolicyId)
		assert.Equal(t, "high", results[0].ResultThreshold)

		assert.Len(t, results[0].RuleResults, 3)
		assert.Equal(t, int64(20), results[0].RuleResults[0].Score)
		assert.False(t, results[0].RuleResults[0].Stop)
		assert.Equal(t, int64(40), results[0].RuleResults[1].Score)
		assert.False(t, results[0].RuleResults[1].Stop)
		assert.Equal(t, int64(100), results[0].RuleResults[2].Score)
		assert.False(t, results[0].RuleResults[2].Stop)

		err = msgs[0].Ack()
		assert.NoError(t, err)
	})

	// Test: Valutazione di una policy con stop
	t.Run("EvaluatePolicyWithStop", func(t *testing.T) {
		sub, err := js.PullSubscribe(cfg.NatsOutputSubject, "test-consumer")
		assert.NoError(t, err)

		input := map[string]interface{}{
			"value": 40,
		}
		inputData, _ := json.Marshal(input)

		_, err = js.Publish(cfg.NatsInputSubject, inputData)
		assert.NoError(t, err)

		msgs, err := sub.Fetch(1, nats.MaxWait(5*time.Second))
		assert.NoError(t, err)
		assert.Len(t, msgs, 1)

		var policyResults api.PolicyResults
		err = proto.Unmarshal(msgs[0].Data, &policyResults)
		assert.NoError(t, err)

		results := policyResults.Results

		assert.Len(t, results, 1)
		assert.Equal(t, "test_policy", results[0].PolicyId)
		assert.Equal(t, "high", results[0].ResultThreshold)

		assert.Len(t, results[0].RuleResults, 3) // Ora dovremmo avere risultati per tutte e 3 le regole
		assert.Equal(t, int64(40), results[0].RuleResults[0].Score)
		assert.False(t, results[0].RuleResults[0].Stop)
		assert.True(t, results[0].RuleResults[0].Executed)
		assert.Equal(t, int64(80), results[0].RuleResults[1].Score)
		assert.True(t, results[0].RuleResults[1].Stop)
		assert.True(t, results[0].RuleResults[1].Executed)
		assert.Equal(t, int64(0), results[0].RuleResults[2].Score)
		assert.False(t, results[0].RuleResults[2].Stop)
		assert.False(t, results[0].RuleResults[2].Executed)

		err = msgs[0].Ack()
		assert.NoError(t, err)
	})
}
