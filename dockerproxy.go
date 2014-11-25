package main

import (
    "github.com/samalba/dockerclient"
    "fmt"
    "log"
    "strings"
    "time"
    "io/ioutil"
    "text/template"
    "os"
    "flag"
)

type ProxyConfig struct {
    Host string
    Port string
    Name string
    Proxy_type string
}

var (
    docker_socket string
    template_path string
    template_dest string
    docker *dockerclient.DockerClient
)

func init() {
    flag.StringVar(&docker_socket, "socket", "unix:///var/run/docker.sock", "Docker socket to use, default unix:///var/run/docker.sock")
    flag.StringVar(&template_path, "template", "haproxy.template", "Template file to use, default haproxy.template")
    flag.StringVar(&template_dest, "output", "haproxy.conf", "Config destination, default haproxy.conf")
    flag.Parse()
}

func check_error(e error){
    if e != nil {
        log.Fatal(e)
        panic(e)
    }
}

func event_callback(event *dockerclient.Event, args ...interface{}) {
    log.Printf("Received Event: %#v\n", *event)
    configs := check_docker()
    err := write_configs(configs)
    if err != nil {
        log.Fatal(err)
    }
}

func check_docker() []ProxyConfig{
    log.Printf("about to list containers")
    containers, err := docker.ListContainers(false, false, "")
    log.Printf("check_docker: checking error")
    check_error(err)

    var configs []ProxyConfig

    for _,container := range containers{
        info, err := docker.InspectContainer(container.Id)
        check_error(err)

        count := 0
        for port, _ := range info.NetworkSettings.Ports{
            var new_config ProxyConfig
            new_config.Host = info.NetworkSettings.IpAddress
            new_config.Name = fmt.Sprintf("%s-%d", info.Name[1:],count)
            port_info := strings.Split(port, "/")
            new_config.Port = port_info[0]
            new_config.Proxy_type = port_info[1]
            configs = append(configs, new_config)
            count += 1
        }
    }

    return configs
}

func write_configs(configs []ProxyConfig) error {
    template_bytes, err := ioutil.ReadFile(template_path)
    template_data := string(template_bytes)

    tmpl := template.New("haproxy-template")
    tmpl, err = tmpl.Parse(template_data)
    if err != nil {
        return err
    }

    //open a new file for writing
    dest_file, err := os.Create(template_dest)
    if err != nil {
        return err
    }
    defer dest_file.Close()

    err = tmpl.Execute(dest_file, configs)
    if err != nil {
        return err
    }

    return err
}

func main(){
    log.Printf("Starting run\n")
    log.Printf("docker socker: %s\n", docker_socket)
    log.Printf("haproxy template: %s\n", template_path)
    log.Printf("haproxy destination: %s\n", template_dest)

    var err error
    docker, err = dockerclient.NewDockerClient(docker_socket, nil)
    check_error(err)

    docker.StartMonitorEvents(event_callback)
    time.Sleep(60 * time.Second)
}
