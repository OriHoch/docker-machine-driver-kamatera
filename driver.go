package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"time"
	"net/http"
    "encoding/json"
    "strings"
    "regexp"
    "bytes"
    "net/url"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnflag"
	mcnssh "github.com/docker/machine/libmachine/ssh"
	"github.com/docker/machine/libmachine/state"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"github.com/go-resty/resty"
	"github.com/sethvargo/go-password/password"
)

type Driver struct {
	*drivers.BaseDriver

	APIClientID string
	APISecret string
	Datacenter string
	Billing string
	Cpu string
	Ram int
	DiskSize int
	Image string

	ServerOptions map[string]interface{}
	ImageID string
	CreateServerCommandId int
	DiskImageId string
	DatacenterName string
	Password string
	KamateraServerId string
}

const (
    defaultDatacenter = "EU"
	defaultBilling = "hourly"
	defaultCpu  = "1B"
	defaultRam = 1024
	defaultDiskSize = 10
	defaultImage = "ubuntu_server_18.04_64-bit"

	flagAPIClientID = "kamatera-api-client-id"
	flagAPISecret = "kamatera-api-secret"
	flagDatacenter = "kamatera-datacenter"
	flagBilling = "kamatera-billing"
	flagCpu = "kamatera-cpu"
	flagRam = "kamatera-ram"
	flagDiskSize = "kamatera-disk-size"
	flagImage = "kamatera-image"
	flagCreateServerCommandId = "kamatera-create-server-command-id"
)

func NewDriver() *Driver {
	return &Driver{
	    Datacenter: defaultDatacenter,
	    Billing: defaultBilling,
	    Cpu: defaultCpu,
	    Ram: defaultRam,
	    DiskSize: defaultDiskSize,
	    Image: defaultImage,
	    CreateServerCommandId: 0,
	    KamateraServerId: "",
	    BaseDriver: &drivers.BaseDriver{
			SSHUser: "root",
			SSHPort: 22,
            // IPAddress      string
            // MachineName    string
            // SSHUser        string
            // SSHPort        int
            // SSHKeyPath     string
            // StorePath      string
            // SwarmMaster    bool
            // SwarmHost      string
            // SwarmDiscovery string
		},
	}
}

func (d *Driver) DriverName() string {
	return "kamatera"
}

func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		mcnflag.StringFlag{
			EnvVar: "KAMATERA_API_CLIENT_ID",
			Name:   flagAPIClientID,
			Usage:  "Kamatera API client ID",
			Value:  "",
		},
		mcnflag.StringFlag{
			EnvVar: "KAMATERA_API_SECRET",
			Name:   flagAPISecret,
			Usage:  "Kamatera API secret",
			Value:  "",
		},
		mcnflag.IntFlag{
			EnvVar: "KAMATERA_CREATE_SERVER_COMMAND_ID",
			Name:   flagCreateServerCommandId,
			Usage:  "Kamatera Create Server Command Id",
			Value:  0,
		},
		mcnflag.StringFlag{
			EnvVar: "KAMATERA_DATACENTER",
			Name:   flagDatacenter,
			Usage:  "Kamatera datacenter",
			Value:  defaultDatacenter,
		},
		mcnflag.StringFlag{
			EnvVar: "KAMATERA_BILLING",
			Name:   flagBilling,
			Usage:  "Kamatera billing method",
			Value:  defaultBilling,
		},
		mcnflag.StringFlag{
			EnvVar: "KAMATERA_CPU",
			Name:   flagCpu,
			Usage:  "Kamatera CPU",
			Value:  defaultCpu,
		},
		mcnflag.IntFlag{
			EnvVar: "KAMATERA_RAM",
			Name:   flagRam,
			Usage:  "Kamatera RAM",
			Value:  defaultRam,
		},
		mcnflag.IntFlag{
			EnvVar: "KAMATERA_DISK_SIZE",
			Name:   flagDiskSize,
			Usage:  "Kamatera disk size",
			Value: defaultDiskSize,
		},
		mcnflag.StringFlag{
			EnvVar: "KAMATERA_IMAGE",
			Name:   flagImage,
			Usage:  "Kamatera image name",
			Value:  defaultImage,
		},
	}
}

func (d *Driver) SetConfigFromFlags(opts drivers.DriverOptions) error {
    d.APIClientID = opts.String(flagAPIClientID)
	d.APISecret = opts.String(flagAPISecret)
	d.Datacenter = opts.String(flagDatacenter)
	d.Billing = opts.String(flagBilling)
	d.Cpu = opts.String(flagCpu)
	d.Ram = opts.Int(flagRam)
	d.DiskSize = opts.Int(flagDiskSize)
	d.Image = opts.String(flagImage)
	d.CreateServerCommandId = opts.Int(flagCreateServerCommandId)

	d.SetSwarmConfigFromFlags(opts)

	if d.APIClientID == "" {
		return errors.Errorf("kamatera requires --%v to be set", flagAPIClientID)
	}

	if d.APISecret == "" {
		return errors.Errorf("kamatera requires --%v to be set", flagAPISecret)
	}

	return nil
}

type KamateraDiskImage struct {
    Description string `json:description`
    Id string `json:id`
    SizeGB int `json:sizeGB`
}

type KamateraServerOptions struct {
    Datacenters map[string]string `json:datacenters`
    Cpu []string `json:cpu`
    Ram []int `json:ram`
    Disk []int `json:disk`
    Billing []string `json:billing`
    DiskImages map[string][]KamateraDiskImage `json:datacenters`
}

type KamateraServerCommandInfo struct {
    Status string `json:status`
    Server string `json:server`
    Description string `json:description`
    Log string `json:log`
}

type KamateraPowerOperationInfo struct {
    Status string `json:status`
}

type KamateraServerListInfo struct {
    Id string `json:id`
    Datacenter string `json:datacenter`
    Name string `json:name`
    Power string `json:power`
}

func IsStringInArray(str string, arr []string) bool {
    for _, n := range arr {if str == n {return true}}; return false
}

func IsIntInArray(i int, arr []int) bool {
    for _, n := range arr {if i == n {return true}}; return false
}

func (d *Driver) PreCreateCheck() error {
    log.Debugf("PreCreateCheck: %s", time.Now())
    if d.CreateServerCommandId != 0 {
        log.Debugf("Skipping pre-create checks, continuing from existing command id = %d", d.CreateServerCommandId)
        return nil
    }
    i := 0
    for {
        log.Debugf("PreCreateCheck (%d): %s", i, time.Now())
        if i > 0 {time.Sleep(time.Duration(i * 6000) * time.Millisecond)}
        i += 1
        resp, err := resty.R().
            SetHeader("AuthClientId", d.APIClientID).
            SetHeader("AuthSecret", d.APISecret).
            SetResult(KamateraServerOptions{}).
            Get("https://console.kamatera.com/service/server")
        if err != nil {return err}
        if resp.StatusCode() != 200 {
            if resp.StatusCode() == 404 {
                return errors.New("Kamatera resource not found, please try again")
            }
            if resp.StatusCode() == 500 {
                return errors.New(fmt.Sprintf("Kamatera API responded with the following error: %s", resp.String()))
            }
            log.Info(resp.String())
            if i >= 10 {
                return errors.New(fmt.Sprintf("Invalid status code: %d", resp.StatusCode()))
            }
            log.Infof("Got invalid status code: %d, retrying... %d/10", resp.StatusCode(), i)
            continue
        }
        res := resp.Result().(*KamateraServerOptions)
        d.DatacenterName = res.Datacenters[d.Datacenter]
        if d.DatacenterName == "" {return errors.New("Invalid datacenter")}
        if ! IsStringInArray(d.Cpu, res.Cpu) {return errors.New("Invalid CPU")}
        if ! IsIntInArray(d.Ram, res.Ram) {return errors.New("Invalid ram")}
        if d.Ram < 999 {return errors.New("Insufficient RAM, Please use at least 1GB of RAM.")}
        if ! IsIntInArray(d.DiskSize, res.Disk) {return errors.New("Invalid disk size")}
        if ! IsStringInArray(d.Billing, res.Billing) {return errors.New("Invalid billing")}
        diskImages := res.DiskImages[d.Datacenter]
        for _, diskImage := range diskImages {
            if diskImage.Description == d.Image {
                d.DiskImageId = diskImage.Id
                break
            }
        }
        if d.DiskImageId == "" {return errors.New(fmt.Sprintf("Invalid disk image: %s", d.Image))}
        return nil
    }
}

func (d *Driver) Create() error {
    log.Debugf("Create: %s", time.Now())
    if d.CreateServerCommandId == 0 {
        log.Infof("Creating Kamatera server...")
        log.Infof("Datacenter: %s", d.DatacenterName)
        log.Infof("Cpu: %s", d.Cpu)
        log.Infof("Ram: %d", d.Ram)
        log.Infof("Disk Size (GB): %d", d.DiskSize)
        log.Infof("Disk Image: %s %s", d.Image, d.DiskImageId)
        password, err := password.Generate(12, 3, 0, false, false)
        if err != nil {return err}
        d.Password = password
        qs := fmt.Sprintf("datacenter=%s&name=%s&password=%s&cpu=%s&ram=%d&billing=%s&disk_size_0=%d&disk_src_0=%s&network_name_0=%s&power=1&managed=0&backup=0", url.PathEscape(d.Datacenter), url.PathEscape(d.MachineName), url.PathEscape(d.Password), url.PathEscape(d.Cpu), d.Ram, url.PathEscape(d.Billing), d.DiskSize, strings.Replace(url.PathEscape(d.DiskImageId), ":", "%3A", -1), "wan")
        log.Debugf("https://console.kamatera.com/service/server?%s", qs)
        payload := strings.NewReader(qs)
        i := 0
        for {
            log.Debugf("Create (%d): %s", i, time.Now())
            if i > 0 {
                log.Debugf("Retry %d / 10", i)
                time.Sleep(time.Duration(i * 6000) * time.Millisecond)
            }
            i += 1
            req, err := http.NewRequest("POST", "https://console.kamatera.com/service/server", payload)
            req.Header.Add("User-Agent", "docker-machine-driver-kamatera/v0.0.0")
            req.Header.Add("Host", "console.kamatera.com")
            req.Header.Add("Accept", "*/*")
            req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
            req.Header.Add("AuthClientId", d.APIClientID)
            req.Header.Add("AuthSecret", d.APISecret)
            r, err := http.DefaultClient.Do(req)
            if err != nil {
                if i >= 10 {
                    return errors.Wrap(err, "Unexpected error")
                } else {
                    log.Debugf("Unexpected error: %s", err)
                    continue
                }
            }
            body, err := ioutil.ReadAll(r.Body)
            if err != nil {
                if i >= 10 {
                    return errors.Wrap(err, "Failed to read Kamatera create server response body")
                } else {
                    log.Debugf("Failed to read Kamatera create server response body: %s", err)
                    continue
                }
            }
            if r.StatusCode != 200 {
                if r.StatusCode == 500 {
                    return errors.New(fmt.Sprintf("Kamatera API responded with the following error: %s", string(body)))
                }
                log.Info(string(body))
                if i >= 10 {
                    return errors.New(fmt.Sprintf("Invalid Kamatera create server response status: %d", r.StatusCode))
                } else {
                    log.Debugf("Got invalid status code: %d", r.StatusCode)
                    continue
                }
            } else {
                log.Debug(string(body))
            }
            var CreateServerResponse []int
            err = json.Unmarshal(body, &CreateServerResponse)
            if err != nil {
                if i >= 10 {
                    return errors.Wrap(err, "Invalid JSON response from Kamatera create server")
                } else {
                    log.Debugf("Failed to parse Kamatera create server response body: %s", err)
                    continue
                }
            }
            defer r.Body.Close()
            d.CreateServerCommandId = CreateServerResponse[0]
            break
        }
    }
    log.Infof("Waiting for Kamatera create server command to complete...")
    log.Infof("You can track progress in the Kamatera console web-ui (Command ID = %d)", d.CreateServerCommandId)
    createServerLog := ""
    for {
        log.Debugf("Create/wait: %s", time.Now())
        time.Sleep(2 * time.Second)
        resp, err := resty.R().SetHeader("AuthClientId", d.APIClientID).
            SetHeader("AuthSecret", d.APISecret).SetResult(KamateraServerCommandInfo{}).
            Get(fmt.Sprintf("https://console.kamatera.com/service/queue/%d", d.CreateServerCommandId))
        if err != nil {return errors.Wrap(err, fmt.Sprintf("Failed to get Kamatera command info (%d)", d.CreateServerCommandId))}
        if resp.StatusCode() == 200 {
            res := resp.Result().(*KamateraServerCommandInfo)
            log.Debugf("%s", res.Status)
            log.Debugf("%s", res.Log)
            createServerLog = res.Log
            if res.Status == "complete" {break}
            if res.Status == "error" {return errors.New("Kamatera create server failed")}
            if res.Status == "cancelled" {return errors.New("Kamatera create server cancelled")}
		} else {
		    if resp.StatusCode() == 404 {
		        log.Infof("Waiting for command to start...")
		        continue
		    }
            if resp.StatusCode() == 500 {
                return errors.New(fmt.Sprintf("Kamatera API responded with the following error: %s", resp.String()))
            }
            log.Infof(resp.String())
            log.Infof("Got invalid status code: %d, retrying...", resp.StatusCode())
        }
	}
	log.Infof("Kamatera create server command completed successfully (%s)", time.Now())
	var pattern = regexp.MustCompile(` ([0-9]+.[0-9]+.[0-9]+.[0-9]+) `)
	d.IPAddress = strings.Trim(pattern.FindString(createServerLog), " ")
	log.Debugf("Server IP = '%s'", d.IPAddress)
	log.Debugf("Generating SSH key...")
    if err := mcnssh.GenerateSSHKey(d.GetSSHKeyPath()); err != nil {
        return errors.Wrap(err, "could not generate ssh key")
    }
    buf, err := ioutil.ReadFile(d.GetSSHKeyPath() + ".pub")
    if err != nil {
        return errors.Wrap(err, "could not read ssh public key")
    }
    pkey := string(buf)
    log.Debugf("Waiting for server status...")
    for {
        log.Debugf("Create/wait-status: %s", time.Now())
        time.Sleep(2 * time.Second)
        srvstate, _ := d.GetState()
        if srvstate == state.Running {break}
    }
    config := &ssh.ClientConfig{
        User: "root",
        Auth: []ssh.AuthMethod{
            ssh.Password(d.Password),
        },
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
    }
    log.Debugf("Copying SSH key to the server and performing initialization")
    for {
        log.Debugf("Create/ssh: %s", time.Now())
        time.Sleep(2 * time.Second)
        client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", d.IPAddress), config)
        if err == nil {
            session, err := client.NewSession()
            if err == nil {
                defer session.Close()
                var b bytes.Buffer
                session.Stdout = &b
                cmd := fmt.Sprintf("bash -c 'mkdir -p .ssh && echo \"%s\" >> .ssh/authorized_keys'", pkey)
                log.Debugf("Running ssh cmd: %s", cmd)
                err = session.Run(cmd)
                if err != nil {return errors.Wrap(err, "Failed to copy SSH key to the Kamatera server")}
                log.Debugf("SSH Initialization completed successfully (%s)", time.Now())
                return nil
            }
        } else {
            log.Debugf("SSH failure (%s): %s", time.Now(), err)
        }
    }
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

func (d *Driver) GetURL() (string, error) {
	if err := drivers.MustBeRunning(d); err != nil {return "", errors.Wrap(err, "could not execute drivers.MustBeRunning")}
	ip, err := d.GetIP()
	if err != nil {return "", errors.Wrap(err, "could not get IP")}
	url := fmt.Sprintf("tcp://%s", net.JoinHostPort(ip, "2376"))
	log.Debug(url)
	return url, nil
}

func (d *Driver) GetState() (state.State, error) {
    power, err := d.getKamateraServerPower()
    if err != nil {
        return state.Starting, nil
    } else if power == "on" {
        return state.Running, nil
    } else if power == "off" {
        return state.Stopped, nil
    } else {
        return state.Error, nil
    }
}

func (d *Driver) getKamateraServerPower() (string, error) {
    i := 0
    for {
        log.Debugf("getKamateraServerPower: %s", time.Now())
        if i > 0 {time.Sleep(2000 + time.Duration(i * 3000) * time.Millisecond)}
        i += 1
        resp, err := resty.R().SetHeader("AuthClientId", d.APIClientID).
            SetHeader("AuthSecret", d.APISecret).Get("https://console.kamatera.com/service/servers")
        if err != nil {return "", errors.Wrap(err, "Failed to get Kamatera server power")}
        if resp.StatusCode() != 200 {
            if resp.StatusCode() == 404 {
                return "", errors.New("Kamatera resource not found")
            }
		    if resp.StatusCode() == 500 {
                return "", errors.New(fmt.Sprintf("Kamatera API responded with the following error: %s", resp.String()))
            }
            log.Info(resp.String())
            if i >= 10 {
                return "", errors.New(fmt.Sprintf("Invalid Kamatera server power status: %d", resp.StatusCode()))
            } else {
                log.Infof("Got invalid status code: %d, retrying... %d/10", resp.StatusCode(), i)
                continue
            }
        }
	log.Debug(resp.String())
        var servers []KamateraServerListInfo
        json.Unmarshal(resp.Body(), &servers)
        serverPower := ""
        for _, server := range servers {
            if server.Name == d.MachineName {
                serverPower = server.Power
                break
            }
        }
        return serverPower, nil
    }
}

func (d *Driver) getKamateraServerId() (string, error) {
    if d.KamateraServerId == "" {
        i := 0
        for {
            log.Debugf("Getting kamatera server id (%s): %d", time.Now(), i)
            if i > 0 {time.Sleep(2000 + time.Duration(i * 3000) * time.Millisecond)}
            i += 1
            resp, err := resty.R().SetHeader("AuthClientId", d.APIClientID).
                SetHeader("AuthSecret", d.APISecret).Get("https://console.kamatera.com/service/servers")
            if err != nil {return "", errors.Wrap(err, "Failed to get Kamatera servers list")}
            if resp.StatusCode() != 200 {
                if resp.StatusCode() == 404 {
                    return "", errors.New("Kamatera resource not found")
                }
                if resp.StatusCode() == 500 {
                    return "", errors.New(fmt.Sprintf("Kamatera API responded with the following error: %s", resp.String()))
                }
                log.Info(resp.String())
                if i >= 10 {
                    return "", errors.New(fmt.Sprintf("Invalid Kamatera servers status: %d", resp.StatusCode()))
                } else {
                    log.Debugf("Got invalid status code: %d, retrying... %d/10", resp.StatusCode(), i)
                    continue
                }
            }
            var servers []KamateraServerListInfo
            json.Unmarshal(resp.Body(), &servers)
            serverId := ""
            for _, server := range servers {
                if server.Name == d.MachineName {
                    serverId = server.Id
                    break
                }
            }
            if serverId == "" {
                return "", errors.Wrap(err, "Failed to find Kamatera server ID")
            } else {
                d.KamateraServerId = serverId
            }
            break
        }
    }
    return d.KamateraServerId, nil
}

func (d *Driver) Remove() error {
    serverId, err := d.getKamateraServerId()
    if err != nil {return err}
    log.Debugf("Removing Kamatera server ID %s", serverId)
    i := 0
    for {
        log.Debugf("Removing server (%s): %d", time.Now(), i)
        if i > 0 {time.Sleep(2000 + time.Duration(i * 3000) * time.Millisecond)}
        i += 1
        resp, err := resty.R().SetHeader("AuthClientId", d.APIClientID).SetHeader("AuthSecret", d.APISecret).
            SetFormData(map[string]string{"confirm":"1","force":"1"}).
            Delete(fmt.Sprintf("https://console.kamatera.com/service/server/%s/terminate", serverId))
        if err != nil {return errors.Wrap(err, "Failed to run terminate operation")}
        if resp.StatusCode() != 200 {
            if resp.StatusCode() == 404 {
                return errors.New("Kamatera resource not found")
            }
            if resp.StatusCode() == 500 {
                return errors.New(fmt.Sprintf("Kamatera API responded with the following error: %s", resp.String()))
            }
            log.Info(resp.String())
            if i >= 10 {
                return errors.New(fmt.Sprintf("Invalid Kamatera remove server status: %d", resp.StatusCode()))
            } else {
                log.Infof("Got invalid status code: %d, retrying... %d/10", resp.StatusCode(), i)
                continue
            }
        }
        var removeServerCommandId int
        err = json.Unmarshal(resp.Body(), &removeServerCommandId)
        if err != nil {return errors.Wrap(err, "Invalid JSON response from Kamatera remove server")}
        log.Infof("Kamatera remove server started, track progress in Kamatera console, command id = %d", removeServerCommandId)
        return nil
    }
}

func (d *Driver) kamateraPower(power string) error {
    serverId, err := d.getKamateraServerId()
    if err != nil {return errors.Wrap(err, "Failed to get server id for power operation")}
    log.Debugf("Initiating power operation %s on Kamatera server ID %s", power, serverId)
    i := 0
    for {
        log.Debugf("Running power operation (%s): %d", time.Now(), i)
        if i > 0 {time.Sleep(2000 + time.Duration(i * 3000) * time.Millisecond)}
        i += 1
        resp, err := resty.R().SetHeader("AuthClientId", d.APIClientID).SetHeader("AuthSecret", d.APISecret).
            SetFormData(map[string]string{"power":power}).
            Put(fmt.Sprintf("https://console.kamatera.com/service/server/%s/power", serverId))
        if err != nil {return errors.Wrap(err, "Failed to run power operation")}
        if resp.StatusCode() != 200 {
            if resp.StatusCode() == 404 {
                return errors.New("Kamatera resource not found")
            }
            if resp.StatusCode() == 500 {
                return errors.New(fmt.Sprintf("Kamatera API responded with the following error: %s", resp.String()))
            }
            log.Infof(resp.String())
            if i >= 10 {
                return errors.New(fmt.Sprintf("Invalid Kamatera power operation status: %d", resp.StatusCode()))
            }
            log.Infof("Got invalid status code: %d, retrying... %d/10", resp.StatusCode(), i)
            continue
        }
        var powerOperationCommandId int
        err = json.Unmarshal(resp.Body(), &powerOperationCommandId)
        if err != nil {return errors.Wrap(err, "Invalid JSON response from Kamatera power operation")}
        log.Info("Waiting for Kamatera power operation to complete")
        log.Infof("track progress in Kamatera console, command id = %d", powerOperationCommandId)
        for {
            log.Debugf("Waiting for power operation (%s)", time.Now())
            time.Sleep(2000 * time.Millisecond)
            resp, err := resty.R().SetHeader("AuthClientId", d.APIClientID).
                SetHeader("AuthSecret", d.APISecret).SetResult(KamateraPowerOperationInfo{}).
                Get(fmt.Sprintf("https://console.kamatera.com/service/queue/%d", powerOperationCommandId))
            if err != nil {return errors.Wrap(err, fmt.Sprintf("Failed to get Kamatera command info (%d)", powerOperationCommandId))}
            if resp.StatusCode() != 200 {
                if resp.StatusCode() == 500 {
                    return errors.New(fmt.Sprintf("Kamatera API responded with the following error: %s", resp.String()))
                }
                log.Info(resp.String())
                if i >= 10 {
                    return errors.New(fmt.Sprintf("Invalid Kamatera power operation wait status: %d", resp.StatusCode()))
                } else {
                    log.Infof("Got invalid status code: %d, retrying...", resp.StatusCode())
                    if resp.StatusCode() == 500 {
                        return d.kamateraPower(power)
                    } else {
                        continue
                    }
                }
            }
            res := resp.Result().(*KamateraPowerOperationInfo)
            log.Debugf("%s", res.Status)
            if res.Status == "complete" {
	            log.Infof("Kamatera power operation completed successfully")
                return nil
            }
            if res.Status == "error" {return errors.New("Kamatera power operation failed")}
            if res.Status == "cancelled" {return errors.New("Kamatera power operation cancelled")}
        }
    }
}

func (d *Driver) Restart() error {
    return d.kamateraPower("restart")
}

func (d *Driver) Start() error {
    return d.kamateraPower("on")
}

func (d *Driver) Stop() error {
    return d.kamateraPower("off")
}

func (d *Driver) Kill() error {
    return d.Stop()
}
