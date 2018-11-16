package main

import (
	// "context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"
	"net/http"
    "encoding/json"
    // "net/url"
    "strings"
    "regexp"
    "bytes"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnflag"
	"github.com/docker/machine/libmachine/mcnutils"
	mcnssh "github.com/docker/machine/libmachine/ssh"
	"github.com/docker/machine/libmachine/state"
	// "github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"github.com/go-resty/resty"
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
	Password string

	ServerOptions map[string]interface{}
	ImageID string
	CreateServerCommandId int
	DiskImageId string
	DatacenterName string
}

const (
    defaultDatacenter = "EU"
	defaultBilling = "hourly"
	defaultCpu  = "1B"
	defaultRam = 512
	defaultDiskSize = 10
	defaultImage = "ubuntu_server_16.04_64-bit"

	flagAPIClientID = "kamatera-api-client-id"
	flagAPISecret = "kamatera-api-secret"
	flagPassword = "kamatera-password"
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
		mcnflag.StringFlag{
			EnvVar: "KAMATERA_PASSWORD",
			Name:   flagPassword,
			Usage:  "Kamatera Server Password",
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
	d.Password = opts.String(flagPassword)
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

	if d.Password == "" {
		return errors.Errorf("kamatera requires --%v to be set", flagPassword)
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

func IsStringInArray(str string, arr []string) bool {
    for _, n := range arr {if str == n {return true}}; return false
}

func IsIntInArray(i int, arr []int) bool {
    for _, n := range arr {if i == n {return true}}; return false
}

func (d *Driver) PreCreateCheck() error {
    if d.CreateServerCommandId != 0 {
        log.Debugf("Skipping pre-create checks, continuing from existing command id = %d", d.CreateServerCommandId)
        return nil
    }
    resp, err := resty.R().
        SetHeader("AuthClientId", d.APIClientID).
        SetHeader("AuthSecret", d.APISecret).
        SetResult(KamateraServerOptions{}).
        Get("https://console.kamatera.com/service/server")
    if err != nil {return err}
    res := resp.Result().(*KamateraServerOptions)
    d.DatacenterName = res.Datacenters[d.Datacenter]
    if d.DatacenterName == "" {return errors.New("Invalid datacenter")}
    if ! IsStringInArray(d.Cpu, res.Cpu) {return errors.New("Invalid CPU")}
    if ! IsIntInArray(d.Ram, res.Ram) {return errors.New("Invalid ram")}
    if ! IsIntInArray(d.DiskSize, res.Disk) {return errors.New("Invalid disk size")}
    if ! IsStringInArray(d.Billing, res.Billing) {return errors.New("Invalid billing")}
    diskImages := res.DiskImages[d.Datacenter]
    for _, diskImage := range diskImages {
        if diskImage.Description == d.Image {
            d.DiskImageId = diskImage.Id
            break
        }
    }
    if d.DiskImageId == "" {return errors.New("Invalid disk image")}
    return nil
}

func (d *Driver) Create() error {
    if d.CreateServerCommandId == 0 {
        log.Infof("Creating Kamatera server...")
        log.Debugf("Datacenter: %s", d.DatacenterName)
        log.Debugf("Cpu: %s", d.Cpu)
        log.Debugf("Ram: %d", d.Ram)
        log.Debugf("Disk Size (GB): %d", d.DiskSize)
        log.Debugf("Disk Image: %s %s", d.Image, d.DiskImageId)
        qs := fmt.Sprintf("datacenter=%s&name=%s&password=%s&cpu=%s&ram=%s&billing=%s&disk_size_0=%s&disk_src_0=%s&network_name_0=%s&power=1&managed=0&backup=0", d.Datacenter, d.MachineName, d.Password, d.Cpu, fmt.Sprintf("%d", d.Ram), d.Billing, fmt.Sprintf("%d", d.DiskSize), strings.Replace(d.DiskImageId, ":", "%3A", -1), "wan")
        log.Debugf(qs)
        payload := strings.NewReader(qs)
        req, err := http.NewRequest("POST", "https://console.kamatera.com/service/server", payload)
        if err != nil {return err}
        req.Header.Add("User-Agent", "docker-machine-driver-kamatera/v0.0.0")
        req.Header.Add("Host", "console.kamatera.com")
        req.Header.Add("Accept", "*/*")
        req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
        req.Header.Add("AuthClientId", d.APIClientID)
        req.Header.Add("AuthSecret", d.APISecret)
        r, err := http.DefaultClient.Do(req)
        if err != nil {return err}
        if r.StatusCode != 200 {
            log.Debugf("%s", r.Body)
            return errors.New("Kamatera create server failed")
        }
        body, err := ioutil.ReadAll(r.Body)
        if err != nil {return err}
        var CreateServerResponse []int
        err = json.Unmarshal(body, &CreateServerResponse)
        if err != nil {return err}
        defer r.Body.Close()
        d.CreateServerCommandId = CreateServerResponse[0]
    }
    log.Debugf("Kamatera create server command ID = %d", d.CreateServerCommandId)
    createServerLog := ""
    for {
        resp, err := resty.R().SetHeader("AuthClientId", d.APIClientID).
            SetHeader("AuthSecret", d.APISecret).SetResult(KamateraServerCommandInfo{}).
            Get(fmt.Sprintf("https://console.kamatera.com/service/queue/%d", d.CreateServerCommandId))
        if err != nil {return err}
        res := resp.Result().(*KamateraServerCommandInfo)
        log.Debugf("%s", res.Status)
        log.Debugf("%s", res.Log)
        createServerLog = res.Log
        if res.Status == "complete" {break}
        if res.Status == "error" {return errors.New("Kamatera create server failed")}
        if res.Status == "cancelled" {return errors.New("Kamatera create server cancelled")}
		time.Sleep(1 * time.Second)
	}
	log.Debugf("Kamatera create server complete (%d)", d.CreateServerCommandId)
	var pattern = regexp.MustCompile(` ([0-9]+.[0-9]+.[0-9]+.[0-9]+) `)
	d.IPAddress = strings.Trim(pattern.FindString(createServerLog), " ")
	log.Debugf("Server IP = '%s'", d.IPAddress)
	log.Debugf("Generating SSH key...")
    if err := mcnssh.GenerateSSHKey(d.GetSSHKeyPath()); err != nil {
        return errors.Wrap(err, "could not generate ssh key")
    }
    log.Debugf("Waiting for SSH access...")
    for {
        srvstate, _ := d.GetState()
        if srvstate == state.Running {break}
        time.Sleep(1 * time.Second)
    }
    buf, err := ioutil.ReadFile(d.GetSSHKeyPath() + ".pub")
    if err != nil {
        return errors.Wrap(err, "could not read ssh public key")
    }
    pkey := string(buf)
    config := &ssh.ClientConfig{
        User: "root",
        Auth: []ssh.AuthMethod{
            ssh.Password(d.Password),
        },
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
    }
    log.Debugf("Copying SSH key to the server")
    client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", d.IPAddress), config)
    if err != nil {return err}
    session, err := client.NewSession()
    if err != nil {return err}
    defer session.Close()
    var b bytes.Buffer
    session.Stdout = &b
    cmd := fmt.Sprintf("bash -c 'mkdir -p .ssh && echo \"%s\" >> .ssh/authorized_keys'", pkey)
    log.Debugf(cmd)
    err = session.Run(cmd)
    if err != nil {return err}
    return nil
}

func (d *Driver) destroyDanglingKey() {
    /*
	if d.danglingKey && !d.IsExistingKey && d.KeyID != 0 {
		key, err := d.getKey()
		if err != nil {
			log.Errorf("could not get key: %v", err)
			return
		}

		if _, err := d.getClient().SSHKey.Delete(context.Background(), key); err != nil {
			log.Errorf("could not delete ssh key: %v", err)
			return
		}
		d.KeyID = 0
	}
	*/
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

func (d *Driver) GetURL() (string, error) {
	if err := drivers.MustBeRunning(d); err != nil {
		return "", errors.Wrap(err, "could not execute drivers.MustBeRunning")
	}

	ip, err := d.GetIP()
	if err != nil {
		return "", errors.Wrap(err, "could not get IP")
	}

	return fmt.Sprintf("tcp://%s", net.JoinHostPort(ip, "2376")), nil
}

func (d *Driver) GetState() (state.State, error) {
    config := &ssh.ClientConfig{
        User: "root",
        Auth: []ssh.AuthMethod{
            ssh.Password(d.Password),
        },
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
    }
    client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", d.IPAddress), config)
    if err == nil {
        session, err := client.NewSession()
        if err == nil {
            defer session.Close()
            var b bytes.Buffer
            session.Stdout = &b
            err := session.Run("/usr/bin/whoami")
            if err == nil {
                log.Debugf(b.String())
                return state.Running, nil
            } else {
                return state.Starting, nil
            }
        } else {
            return state.Starting, nil
        }
    } else {
        return state.Starting, nil
    }
    // return state.Stopped, nil
}

func (d *Driver) Remove() error {/*
	if d.ServerID != 0 {
		srv, err := d.getServerHandle()
		if err != nil {
			return errors.Wrap(err, "could not get server handle")
		}

		if srv == nil {
			log.Infof(" -> Server does not exist anymore")
		} else {
			log.Infof(" -> Destroying server %s[%d] in...", srv.Name, srv.ID)

			if _, err := d.getClient().Server.Delete(context.Background(), srv); err != nil {
				return errors.Wrap(err, "could not delete server")
			}
		}
	}

	if !d.IsExistingKey && d.KeyID != 0 {
		key, err := d.getKey()
		if err != nil {
			return errors.Wrap(err, "could not get ssh key")
		}
		if key == nil {
			log.Infof(" -> SSH key does not exist anymore")
			return nil
		}

		log.Infof(" -> Destroying SSHKey %s[%d]...", key.Name, key.ID)

		if _, err := d.getClient().SSHKey.Delete(context.Background(), key); err != nil {
			return errors.Wrap(err, "could not delete ssh key")
		}
	}
*/
	return nil
}

func (d *Driver) Restart() error {/*
	srv, err := d.getServerHandle()
	if err != nil {
		return errors.Wrap(err, "could not get server handle")
	}
	if srv == nil {
		return errors.New("server not found")
	}

	act, _, err := d.getClient().Server.Reboot(context.Background(), srv)
	if err != nil {
		return errors.Wrap(err, "could not reboot server")
	}

	log.Infof(" -> Rebooting server %s[%d] in %s[%d]...", srv.Name, srv.ID, act.Command, act.ID)

	return d.waitForAction(act)
	*/
	return nil
}

func (d *Driver) Start() error {/*
	srv, err := d.getServerHandle()
	if err != nil {
		return errors.Wrap(err, "could not get server handle")
	}
	if srv == nil {
		return errors.New("server not found")
	}

	act, _, err := d.getClient().Server.Poweron(context.Background(), srv)
	if err != nil {
		return errors.Wrap(err, "could not power on server")
	}

	log.Infof(" -> Starting server %s[%d] in %s[%d]...", srv.Name, srv.ID, act.Command, act.ID)

	return d.waitForAction(act)
	*/
	return nil
}

func (d *Driver) Stop() error {
    /*
	srv, err := d.getServerHandle()
	if err != nil {
		return errors.Wrap(err, "could not get server handle")
	}
	if srv == nil {
		return errors.New("server not found")
	}

	act, _, err := d.getClient().Server.Shutdown(context.Background(), srv)
	if err != nil {
		return errors.Wrap(err, "could not shutdown server")
	}

	log.Infof(" -> Shutting down server %s[%d] in %s[%d]...", srv.Name, srv.ID, act.Command, act.ID)

	return d.waitForAction(act)
	*/
	return nil
}

func (d *Driver) Kill() error {/*
	srv, err := d.getServerHandle()
	if err != nil {
		return errors.Wrap(err, "could not get server handle")
	}
	if srv == nil {
		return errors.New("server not found")
	}

	act, _, err := d.getClient().Server.Poweroff(context.Background(), srv)
	if err != nil {
		return errors.Wrap(err, "could not poweroff server")
	}

	log.Infof(" -> Powering off server %s[%d] in %s[%d]...", srv.Name, srv.ID, act.Command, act.ID)

	return d.waitForAction(act)
	*/
	return nil
}

/*func (d *Driver) getClient() *hcloud.Client {
	return hcloud.NewClient(hcloud.WithToken(d.AccessToken))
}*/

func (d *Driver) copySSHKeyPair(src string) error {
	if err := mcnutils.CopyFile(src, d.GetSSHKeyPath()); err != nil {
		return errors.Wrap(err, "could not copy ssh key")
	}

	if err := mcnutils.CopyFile(src+".pub", d.GetSSHKeyPath()+".pub"); err != nil {
		return errors.Wrap(err, "could not copy ssh public key")
	}

	if err := os.Chmod(d.GetSSHKeyPath(), 0600); err != nil {
		return errors.Wrap(err, "could not set permissions on the ssh key")
	}

	return nil
}
/*
func (d *Driver) getLocation() (*hcloud.Location, error) {
	if d.cachedLocation != nil {
		return d.cachedLocation, nil
	}

	location, _, err := d.getClient().Location.GetByName(context.Background(), d.Location)
	if err != nil {
		return location, errors.Wrap(err, "could not get location by name")
	}
	d.cachedLocation = location
	return location, nil
}

func (d *Driver) getType() (string, error) {
	if d.cachedType != "" {return d.cachedType, nil}
	serverOpts, err := d.getKamateraServerOptions()
	if err != nil {return "", err}
    log.Debugf(serverOpts)

	stype, _, err := d.getClient().ServerType.GetByName(context.Background(), d.Type)
	if err != nil {
		return stype, errors.Wrap(err, "could not get type by name")
	}
	d.cachedType = stype
	return "", nil
}

func (d *Driver) getImage() (*hcloud.Image, error) {
	if d.cachedImage != nil {
		return d.cachedImage, nil
	}

	var image *hcloud.Image
	var err error

	if d.ImageID != 0 {
		image, _, err = d.getClient().Image.GetByID(context.Background(), d.ImageID)
		if err != nil {
			return image, errors.Wrap(err, fmt.Sprintf("could not get image by id %v", d.ImageID))
		}
	} else {
		image, _, err = d.getClient().Image.GetByName(context.Background(), d.Image)
		if err != nil {
			return image, errors.Wrap(err, fmt.Sprintf("could not get image by name %v", d.Image))
		}
	}

	d.cachedImage = image
	return image, nil
}

func (d *Driver) getKey() (*hcloud.SSHKey, error) {
	if d.cachedKey != nil {
		return d.cachedKey, nil
	}

	stype, _, err := d.getClient().SSHKey.GetByID(context.Background(), d.KeyID)
	if err != nil {
		return stype, errors.Wrap(err, "could not get sshkey by ID")
	}
	d.cachedKey = stype
	return stype, nil
}

func (d *Driver) getServerHandle() (*hcloud.Server, error) {
	if d.cachedServer != nil {
		return d.cachedServer, nil
	}

	if d.ServerID == 0 {
		return nil, errors.New("server ID was 0")
	}

	srv, _, err := d.getClient().Server.GetByID(context.Background(), d.ServerID)
	if err != nil {
		return nil, errors.Wrap(err, "could not get client by ID")
	}

	d.cachedServer = srv
	return srv, nil
}

func (d *Driver) waitForAction(a *hcloud.Action) error {
	for {
		act, _, err := d.getClient().Action.GetByID(context.Background(), a.ID)
		if err != nil {
			return errors.Wrap(err, "could not get client by ID")
		}

		if act.Status == hcloud.ActionStatusSuccess {
			log.Debugf(" -> finished %s[%d]", act.Command, act.ID)
			break
		} else if act.Status == hcloud.ActionStatusRunning {
			log.Debugf(" -> %s[%d]: %d %%", act.Command, act.ID, act.Progress)
		} else if act.Status == hcloud.ActionStatusError {
			return act.Error()
		}

		time.Sleep(1 * time.Second)
	}
	return nil
}
*/