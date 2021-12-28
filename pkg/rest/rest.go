package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	qdnqnv1 "clientmgr.io/tutorial/api/v1"
)

type clientsOnboard struct {
	ClientsOnboard int64 `json:"clientsOnboard,omitempty"`
}

func BindClient(clientResource *qdnqnv1.Client, IP string) bool {
	data := map[string]string{
		"clientId": clientResource.Spec.ClientId,
		"IP":       IP,
	}

	fmt.Println("REST: " + "http://" + IP + ":8080/addClient")

	jsonValue, _ := json.Marshal(data)
	resp, err := http.Post("http://"+IP+":8080/addClient", "application/json", bytes.NewBuffer(jsonValue))

	if err == nil && resp.StatusCode == http.StatusOK {
		return true
	} else {
		return false
	}
}

func HasClients(clientResource *qdnqnv1.Client, IP string) bool {
	resp, err := http.Get("http://" + IP + ":8080/hasClients")

	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var co = clientsOnboard{}
	json.Unmarshal(body, &co)

	if err == nil && co.ClientsOnboard > 0 {
		return true
	} else {
		return false
	}
}

func GetClient(clientResource *qdnqnv1.Client, IP string) bool {
	resp, err := http.Get("http://" + IP + ":8080/client/" + clientResource.Spec.ClientId)

	if err == nil && resp.StatusCode == http.StatusOK {
		return true
	} else {
		return false
	}
}
