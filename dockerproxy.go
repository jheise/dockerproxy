package main

import (
    "github.com/samalba/dockerclient"
    "fmt"
    "log"
    "strings"
    "io/ioutil"
    "text/template"
    "os"
    "os/signal"
    "os/exec"
    "flag"
)

type ProxyConfig struct {
    Hosts []string
    Port string
    Name string
    Proxy_type string
}

type ProxyConfigs []ProxyConfig

var (
    docker_socket string
    template_path string
    template_dest string
    template_file string
    template_default string
    docker *dockerclient.DockerClient
    container_count int
    interrupt chan os.Signal
)

func init() {
    flag.StringVar(&docker_socket, "socket", "unix:///var/run/docker.sock", "Docker socket to use, default unix:///var/run/docker.sock")
    flag.StringVar(&template_path, "templates", "/code", "Directory that holds configs, default /code")
    flag.StringVar(&template_dest, "output", "/etc/haproxy/haproxy.cfg", "Config destination, default haproxy.conf")
    flag.Parse()

    template_file = fmt.Sprintf("%s/haproxy.template", template_path)
    template_default = fmt.Sprintf("%s/haproxy.default", template_path)
}

func check_error(e error){
    if e != nil {
        log.Fatal(e)
        panic(e)
    }
}

func generate_default() error{
    cpCmd := exec.Command("cp", template_default, template_dest)
    err := cpCmd.Run()
    return err
}

func handle_configs() {
    configs := check_docker()
    var err error
    if len(configs) > 0 {
        err = configs.write_configs()
    }else{
        log.Println("No forwarding containers active, switching to default")
        generate_default()
    }
    check_error(err)
    restart := exec.Command("/etc/init.d/haproxy", "reload")
    err = restart.Run()
    check_error(err)
}

func event_callback(event *dockerclient.Event, args ...interface{}) {
    log.Printf("Received Event: %#v\n", *event)
    if event.Status == "stop" || event.Status == "start" {
        handle_configs()
    }
}

func check_docker() ProxyConfigs{
    containers, err := docker.ListContainers(false, false, "")
    check_error(err)

    var configs ProxyConfigs

    for _,container := range containers{
        info, err := docker.InspectContainer(container.Id)
        check_error(err)

        for _, entry := range info.Config.Env{
            if entry == "FORWARD=YES" {
                log.Println("Forwarding", info.Name[1:])

                count := 0
                for port, _ := range info.NetworkSettings.Ports{

                    new_host := info.NetworkSettings.IpAddress
                    new_name := fmt.Sprintf("%s-%d", info.Name[1:],count)
                    port_info := strings.Split(port, "/")
                    new_port := port_info[0]
                    found := false

                    for i := range configs{
                        if configs[i].Port == new_port{
                            found = true
                            configs[i].Hosts = append(configs[i].Hosts, new_host)
                        }
                    }

                    if !found {
                        new_config := ProxyConfig{Port:new_port, Proxy_type: "tcp", Name:new_name}
                        new_config.Hosts = append(new_config.Hosts, new_host)
                        configs = append(configs, new_config)
                    }
                    count += 1
                }
            }
        }
    }

    return configs
}

func (configs ProxyConfigs) write_configs() error {

    template_bytes, err := ioutil.ReadFile(template_file)
    if err != nil {
        return err
    }
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
    log.Printf("docker socket: %s\n", docker_socket)
    log.Printf("haproxy template: %s\n", template_path)
    log.Printf("haproxy destination: %s\n", template_dest)

    var err error
    docker, err = dockerclient.NewDockerClient(docker_socket, nil)
    check_error(err)

    handle_configs()

    interrupt = make(chan os.Signal, 1)
    signal.Notify(interrupt, os.Interrupt, os.Kill)

    docker.StartMonitorEvents(event_callback)
    s := <-interrupt
    log.Printf("Caught %s, cleaning up...\n", s)
}
