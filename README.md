# ComposeGen

**go-composegen** is a Go library that generates a Docker Compose file from existing Docker containers. This library inspects running containers, gathers configuration details, and creates a `docker-compose.yml` file that can be used to redeploy the containers. It supports filtering containers based on names, generating associated networks and volumes, and exporting the result in YAML format.

## Features

- **Generate Docker Compose**: Create a `docker-compose.yml` file from currently running Docker containers.
- **Customizable Filtering**: Filter containers by name using regular expressions.
- **Network and Volume Support**: Automatically includes network and volume configurations for each container.
- **Flexible Volume Handling**: Choose to include bind mounts, Docker volumes, or both.
- **YAML Export**: Outputs the Compose configuration in YAML format.

## Usage

1. Import ComposeGen in your Go project:

```go
import "github.com/nodeharbor/go-composegen"
```

2. Use the `GenerateComposeFile` function to generate a Compose file:

```go
composeFile, err := composegen.GenerateComposeFile(cli, true, "my-filter", true)
if err != nil {
    log.Fatalf("Error generating compose file: %v", err)
}
fmt.Println(composeFile)
```

- `cli`: Docker client.
- `includeAllContainers`: Whether to include all containers' network information.
- `containerFilter`: Regular expression to filter containers by name.
- `createVolumes`: Whether to include volumes.

## Example

```go
cli, _ := client.NewClientWithOpts(client.FromEnv)
composeFile, err := composegen.GenerateComposeFile(cli, false, "", true)
if err != nil {
    log.Fatalf("Error: %v", err)
}
fmt.Println(composeFile)
```

This will generate a Docker Compose file for all running containers and include any volumes used.

## Installation

To install the library, use `go get`:

```bash
go get github.com/nodeharbor/go-composegen
```

## Contact

For any inquiries or support, please contact the maintainer at:  
**hello@nodeharbor.io**

## License

This project is licensed under the MIT License.
