package main

import (
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"fmt"
)

func main() {

	cfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
		Transport: client.DefaultTransport,
	}

	c, err := client.New(cfg)
	if err != nil {
		// handle error
	}

	kAPI := client.NewKeysAPI(c)

	// create a new key /foo with the value "bar"
	_, err = kAPI.Create(context.Background(), "/foo", "bar")
	if err != nil {
		// handle error
	}

	// delete the newly created key only if the value is still "bar"
	//_, err = kAPI.Delete(context.Background(), "/foo", &DeleteOptions{PrevValue: "bar"})
	if err != nil {
		// handle error
	}

	kAPI.Set(context.Background(), "foo2", "bar2", nil)
	response, _ := kAPI.Get(context.Background(), "foo2", nil)
	fmt.Printf("Obtained: %s", response)

}
