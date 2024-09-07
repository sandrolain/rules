# Rules Engine

This project implements a flexible and extensible rules engine using the Common Expression Language (CEL) for policy and rule evaluation. It provides a gRPC service for managing policies and rules, and integrates with NATS for event-driven policy evaluation.

## Features

- Define and manage policies and rules using CEL expressions
- gRPC API for policy management (add, list, get, delete)
- NATS integration for event-driven policy evaluation
- Flexible configuration using environment variables
- Comprehensive test coverage

## Getting Started

### Prerequisites

- Go 1.16 or later
- NATS server
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
- `NATS_SUBJECT`: NATS subject to subscribe to (default: "input")
- `LOG_LEVEL`: Logging level (debug, info, warn, error; default: info)
- `GRPC_PORT`: gRPC server port (default: "50051")

### Running the Application
