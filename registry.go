package docker

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/bborbe/io/util"
	"github.com/golang/glog"
)

type RegistryUsername string

func (r RegistryUsername) String() string {
	return string(r)
}

func (r RegistryUsername) Validate() error {
	if len(r) == 0 {
		return errors.New("username empty")
	}
	return nil
}

type RegistryToken string

func (r RegistryToken) String() string {
	return string(r)
}

type RegistryPassword string

func (r RegistryPassword) String() string {
	return string(r)
}

func (r RegistryPassword) Validate() error {
	if len(r) == 0 {
		return errors.New("password empty")
	}
	return nil
}

func RegistryPasswordFromFile(path string) (RegistryPassword, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return RegistryPassword(strings.TrimSpace(string(content))), nil
}

type RegistryName string

func (r RegistryName) String() string {
	return string(r)
}

func (r RegistryName) Url() string {
	if r.IsDockerHub() {
		return "https://hub.docker.com"
	}
	return fmt.Sprintf("https://%s", r.String())
}

func (r RegistryName) Validate() error {
	if len(r) == 0 {
		return errors.New("registry empty")
	}
	return nil
}

func (r RegistryName) IsDockerHub() bool {
	return "docker.io" == r.String()
}

type Registry struct {
	Name     RegistryName
	Token    RegistryToken
	Username RegistryUsername
	Password RegistryPassword
}

func (r *Registry) ReadCredentialsFromDockerConfig() error {
	dockerConfig := "~/.docker/config.json"
	path, err := util.NormalizePath(dockerConfig)
	if err != nil {
		return errors.Wrapf(err, "normalize path %s failed", dockerConfig)
	}
	file, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "open file %s failed", path)
	}
	return r.CredentialsFromDockerConfig(file)
}

func (r *Registry) CredentialsFromDockerConfig(reader io.Reader) error {
	var data struct {
		Domain map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.NewDecoder(reader).Decode(&data); err != nil {
		return errors.Wrap(err, "decode json failed")
	}
	auth, ok := data.Domain[nameToDomain(r.Name)]
	if !ok {
		return errors.Errorf("domain %s not found in docker config", r.Name)
	}
	value, err := base64.StdEncoding.DecodeString(auth.Auth)
	if err != nil {
		return errors.Wrap(err, "base64 decode auth failed")
	}
	parts := strings.SplitN(string(value), ":", 2)
	if len(parts) != 2 {
		return errors.New("split auth failed")
	}
	r.Username = RegistryUsername(parts[0])
	r.Password = RegistryPassword(parts[1])
	return nil
}

func nameToDomain(name RegistryName) string {
	if name.IsDockerHub() {
		return "https://index.docker.io/v1/"
	}
	return name.String()
}

func (r *Registry) Validate() error {
	if err := r.Name.Validate(); err != nil {
		return err
	}
	if err := r.Username.Validate(); err != nil {
		return err
	}
	if err := r.Password.Validate(); err != nil {
		return err
	}
	return nil
}

func (r *Registry) GetToken() (RegistryToken, error) {
	b := bytes.NewBufferString(fmt.Sprintf(`{"username": "%s", "password": "%s"}`, r.Username, r.Password))
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v2/users/login/", r.Name.Url()), b)
	if err != nil {
		return "", errors.Wrap(err, "create request failed")
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", errors.Errorf("status code %d != 2xx", resp.StatusCode)
	}
	var data struct {
		Token RegistryToken `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", errors.Wrap(err, "decode response failed")
	}
	glog.V(4).Infof("got token: %s", data.Token)
	return data.Token, nil
}

func (r *Registry) SetAuth(req *http.Request) error {
	if r.Name.IsDockerHub() {
		token, err := r.GetToken()
		if err != nil {
			return errors.Wrap(err, "get token failed")
		}
		req.Header.Add("Authorization", fmt.Sprintf("JWT %s", token.String()))
		glog.V(4).Infof("set Authorization header")
	} else if r.Username.String() != "" && r.Password.String() != "" {
		req.SetBasicAuth(r.Username.String(), r.Password.String())
		glog.V(4).Infof("set basic auth")
	}
	return nil
}