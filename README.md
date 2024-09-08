# Rules Engine

This project implements a flexible and extensible rules engine using the Common Expression Language (CEL) for policy and rule evaluation. It provides a NATS-based API for managing policies and rules, and integrates with NATS JetStream for event-driven policy evaluation.

## Features

- Define and manage policies and rules using CEL expressions
- NATS JetStream-based API for policy management (set, list, get, delete)
- Protocol Buffers for message serialization
- protovalidate for request validation
- NATS JetStream integration for event-driven policy evaluation
- Flexible configuration using environment variables
- Comprehensive test coverage

## Getting Started

### Prerequisites

- Go 1.16 or later
- NATS server with JetStream enabled
- Protocol Buffers compiler (protoc)
- buf (for managing Protocol Buffers)

### Installation

1. Clone the repository:

   ```
   git clone https://github.com/sandrolain/rules.git
   cd rules
   ```

2. Install dependencies:

   ```
   go mod download
   ```

3. Generate Protocol Buffer code:

   ```
   make proto
   ```

### Configuration

The application can be configured using environment variables:

- `NATS_URL`: NATS server URL (default: "nats://localhost:4222")
- `NATS_INPUT_SUBJECT`: NATS subject for input messages (default: "rules.engine.input")
- `NATS_OUTPUT_SUBJECT`: NATS subject for output messages (default: "rules.engine.output")
- `NATS_INPUT_STREAM`: NATS JetStream name for input (default: "RULES_INPUT")
- `NATS_OUTPUT_STREAM`: NATS JetStream name for output (default: "RULES_OUTPUT")
- `LOG_LEVEL`: Logging level (debug, info, warn, error; default: info)

### Running the Application
