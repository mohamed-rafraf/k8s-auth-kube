package core 

import (
	"github.com/mohamed-rafraf/k8s-auth-kube/util"
	"time"
	"log"

)

func Start() {
	var err error
	VERIFY:
	err=util.Verify()
	if err !=nil{
		log.Println(err)
		time.Sleep(500* time.Millisecond)
		goto VERIFY
	}
    CONNECT:
	err=util.Connect()
	if err !=nil {
		log.Println(err)
		time.Sleep(500* time.Millisecond)
		goto CONNECT
	}
}