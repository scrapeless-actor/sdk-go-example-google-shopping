package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/scrapeless-ai/scrapeless-actor-sdk-go/scrapeless"
	proxyModel "github.com/scrapeless-ai/scrapeless-actor-sdk-go/scrapeless/proxy"
)

func main() {
	actor := scrapeless.New(scrapeless.WithProxy(), scrapeless.WithStorage())
	defer actor.Close()
	var param = &RequestParam{}
	if err := actor.Input(param); err != nil {
		panic(err)
	}
	ctx := context.TODO()
	// Get proxy
	proxy, err := actor.Proxy.Proxy(ctx, proxyModel.ProxyActor{
		Country:         "us",
		SessionDuration: 10,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to get proxy, err:%s", err.Error()))
	}
	shopping := DoShopping(*param, proxy)
	marshal, _ := json.Marshal(shopping)
	kv := actor.Storage.GetKv()
	key := "google-shopping"
	setValue, err := kv.SetValue(ctx, key, string(marshal), 0)
	if err != nil {
		panic(fmt.Sprintf("Failed to set kv, err:%s", err.Error()))
	}
	if setValue {
		fmt.Println("Kv-->set value success")
	}
	value, err := kv.GetValue(ctx, key)
	if err != nil {
		panic(fmt.Sprintf("Failed to get kv, err:%s", err.Error()))
	}
	fmt.Printf("Kv-->get valuse success:\n%s", value)
}
