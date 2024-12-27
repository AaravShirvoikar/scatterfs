# scatterfs

A peer-to-peer distributed file system that enables decentralized file storage and retrieval across multiple nodes.

## Features
- Decentralized file storage with multiple file servers.
- Communication between nodes via a flexible transport.
- Secure data handling with AES encryption.
- Operations: Store, Get, and Delete files.

## Requirements
- **Go**: Ensure you have [Go](https://go.dev/) installed.

## Installation
1. Clone the repository:
   ```bash
   git clone https://github.com/AaravShirvoikar/scatterfs
   cd scatterfs
   ```

2. Build and run the CLI:
   ```bash
   make run
   ```

## Usage
When the CLI starts, it will:
1. Spin up 3 file server nodes locally.
2. Allow you to:
   - **Store**: Save a file to the network.
   - **Get**: Retrieve a file.
   - **Delete**: Remove a file locally from a server.
