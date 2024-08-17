package composegen

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"gopkg.in/yaml.v2"
)

var ignoreValues = []interface{}{nil, "", []interface{}{}, "null", map[string]interface{}{}, "default", 0, ",", "no"}

type Config struct {
	Version  string                            `yaml:"version"`
	Services map[string]map[string]interface{} `yaml:"services"`
	Networks map[string]map[string]interface{} `yaml:"networks,omitempty"`
	Volumes  map[string]map[string]interface{} `yaml:"volumes,omitempty"`
}

func GenerateComposeFile(cli *client.Client, includeAllContainers bool, containerFilter string, createVolumes bool) (string, error) {
	containerNames, err := listContainerNames(cli)
	if err != nil {
		return "", err
	}

	if containerFilter != "" {
		filterRegex, err := regexp.Compile(containerFilter)
		if err != nil {
			return "", fmt.Errorf("invalid filter regex: %v", err)
		}

		filtered := []string{}
		for _, name := range containerNames {
			if filterRegex.MatchString(name) {
				filtered = append(filtered, name)
			}
		}
		containerNames = filtered
	}

	structure := map[string]map[string]interface{}{}
	networks := map[string]map[string]interface{}{}
	volumes := map[string]map[string]interface{}{}

	for _, cname := range containerNames {
		cfile, cNetworks, cVolumes, err := generate(cli, cname, createVolumes)
		if err != nil {
			log.Printf("Error generating config for container %s: %v", cname, err)
			continue
		}

		for key, value := range cfile {
			structure[key] = value
		}

		for key, value := range cNetworks {
			networks[key] = value
		}

		for key, value := range cVolumes {
			volumes[key] = value
		}
	}

	// Include network information if needed
	if includeAllContainers {
		hostNetworks, err := generateNetworkInfo(cli)
		if err != nil {
			return "", fmt.Errorf("error generating network info: %v", err)
		}
		networks = hostNetworks
	}

	config := Config{
		Version:  "3.6",
		Services: structure,
		Networks: networks,
		Volumes:  volumes,
	}

	return render(config)
}

func listContainerNames(cli *client.Client) ([]string, error) {
	containers, err := cli.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var names []string
	for _, container := range containers {
		containerName := container.Names[0][1:]
		names = append(names, containerName)
	}
	return names, nil
}

func generate(cli *client.Client, cname string, createVolumes bool) (map[string]map[string]interface{}, map[string]map[string]interface{}, map[string]map[string]interface{}, error) {
	containers, err := cli.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return nil, nil, nil, err
	}

	var containerID string
	for _, container := range containers {
		if container.Names[0][1:] == cname || container.ID == cname {
			containerID = container.ID
			break
		}
	}

	if containerID == "" {
		return nil, nil, nil, fmt.Errorf("container %s not found", cname)
	}

	cattrs, err := cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return nil, nil, nil, err
	}

	cfile := map[string]map[string]interface{}{}
	ct := map[string]interface{}{}

	values := map[string]interface{}{
		"container_name": cattrs.Name[1:],
		"image":          cattrs.Config.Image,
		"labels":         cattrs.Config.Labels,
		"volumes":        generateVolumes(cattrs, createVolumes),
		"environment":    cattrs.Config.Env,
		"command":        cattrs.Config.Cmd,
		"entrypoint":     cattrs.Config.Entrypoint,
		"working_dir":    cattrs.Config.WorkingDir,
		"user":           cattrs.Config.User,
		"hostname":       cattrs.Config.Hostname,
		"domainname":     cattrs.Config.Domainname,
		"network_mode":   getNetworkMode(cattrs),
		"ports":          getPorts(cattrs),
		"privileged":     cattrs.HostConfig.Privileged,
		"restart":        cattrs.HostConfig.RestartPolicy.Name,
		"tty":            cattrs.Config.Tty,
		"stdin_open":     cattrs.Config.OpenStdin,
	}

	networks := map[string]map[string]interface{}{}
	if len(cattrs.NetworkSettings.Networks) > 0 {
		for networkName := range cattrs.NetworkSettings.Networks {
			networkResource, err := cli.NetworkInspect(context.Background(), networkName, types.NetworkInspectOptions{})
			if err != nil {
				log.Printf("Error inspecting network %s: %v", networkName, err)
				continue
			}

			networks[networkName] = map[string]interface{}{
				"external": !networkResource.Internal,
				"name":     networkName,
			}
		}
	}

	// Removing default or ignored values
	for key, value := range values {
		if !isIgnoredValue(value) {
			ct[key] = value
		}
	}

	cfile[cattrs.Name[1:]] = ct
	return cfile, networks, nil, nil
}

func generateVolumes(cattrs types.ContainerJSON, createVolumes bool) []string {
	var volumes []string
	for _, mount := range cattrs.Mounts {
		var volume string
		if mount.Type == "volume" && createVolumes {
			volume = fmt.Sprintf("%s:%s", mount.Name, mount.Destination)
		} else if mount.Type == "bind" {
			volume = fmt.Sprintf("%s:%s", mount.Source, mount.Destination)
		}
		volumes = append(volumes, volume)
	}
	sort.Strings(volumes)
	return volumes
}

func getNetworkMode(cattrs types.ContainerJSON) string {
	if cattrs.HostConfig.NetworkMode.IsDefault() {
		for name := range cattrs.NetworkSettings.Networks {
			return name
		}
	}
	return string(cattrs.HostConfig.NetworkMode)
}

func getPorts(cattrs types.ContainerJSON) []string {
	var ports []string
	for port, bindings := range cattrs.HostConfig.PortBindings {
		for _, binding := range bindings {
			hostPort := binding.HostPort
			if binding.HostIP != "" {
				hostPort = binding.HostIP + ":" + hostPort
			}
			ports = append(ports, fmt.Sprintf("%s:%s", hostPort, port))
		}
	}
	return ports
}

func isIgnoredValue(value interface{}) bool {
	for _, iv := range ignoreValues {
		if value == iv {
			return true
		}
	}
	return false
}

func listNetworkNames(cli *client.Client) ([]string, error) {
	networks, err := cli.NetworkList(context.Background(), types.NetworkListOptions{})
	if err != nil {
		return nil, err
	}

	var names []string
	for _, network := range networks {
		names = append(names, network.Name)
	}
	return names, nil
}

func generateNetworkInfo(cli *client.Client) (map[string]map[string]interface{}, error) {
	networks := make(map[string]map[string]interface{})
	networkNames, err := listNetworkNames(cli)
	if err != nil {
		return nil, err
	}

	for _, networkName := range networkNames {
		networkResource, err := cli.NetworkInspect(context.Background(), networkName, types.NetworkInspectOptions{})
		if err != nil {
			return nil, err
		}

		values := map[string]interface{}{
			"name":        networkResource.Name,
			"scope":       networkResource.Scope,
			"driver":      networkResource.Driver,
			"enable_ipv6": networkResource.EnableIPv6,
			"internal":    networkResource.Internal,
			"ipam": map[string]interface{}{
				"driver": networkResource.IPAM.Driver,
				"config": networkResource.IPAM.Config,
			},
		}

		networks[networkName] = values
	}

	return networks, nil
}

func render(config Config) (string, error) {
	out, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("error marshaling to YAML: %v", err)
	}
	return string(out), nil
}
