package server

import (
	"fmt"
	"net/http"
	"sync"
	"text/template"
	"time"
)

// index.html resource
var myTemplate *template.Template

// // data for web HTML
// var socketStates []SocketState

// web visualization information
type SocketState struct {
	IP       string
	ID       string
	Dest     string
	State    string
	IsServer bool
	SendFPS  int
	RelayFPS int
	RecvFPS  int
}

var TCPSendCnt, TCPRelayCnt, TCPRecvCnt sync.Map

func FPSCounter() {
	duration := time.Duration(time.Second)
	t := time.NewTicker(duration)
	defer t.Stop()
	for {
		<-t.C
		TCPSockets.Range(func(idWithKey, socket interface{}) bool {
			sendCnt, ok := TCPSendCnt.Load(idWithKey)
			if !ok {
				fmt.Println("\033[1;35m [web.go:] No key in TCPSendCnt", idWithKey, "\033[0m")
				return true
			}
			relayCnt, ok := TCPRelayCnt.Load(idWithKey)
			if !ok {
				fmt.Println("\033[1;35m [web.go:]No key in TCPRelayCnt", idWithKey, "\033[0m")
				return true
			}
			recvCnt, ok := TCPRecvCnt.Load(idWithKey)
			if !ok {
				fmt.Println("\033[1;35m [web.go:]No key in TCPRecvCnt", idWithKey, "\033[0m")
				return true
			}
			_state, _ := TCPStateMap.Load(idWithKey)
			state := _state.(SocketState)
			state.SendFPS = sendCnt.(int)
			state.RelayFPS = relayCnt.(int)
			state.RecvFPS = recvCnt.(int)

			TCPStateMap.Store(idWithKey, state)
			TCPSendCnt.Store(idWithKey, 0)
			TCPRelayCnt.Store(idWithKey, 0)
			TCPRecvCnt.Store(idWithKey, 0)
			return true
		})
	}
}

func initTemplate(fileName string) (err error) {
	myTemplate, err = template.ParseFiles(fileName)
	checkErr(err)
	return err
}

func webHandler(writer http.ResponseWriter, request *http.Request) {
	data := make(map[string]interface{})
	data["title"] = "Data Relay"
	var state []SocketState
	TCPStateMap.Range(func(key, value interface{}) bool {
		state = append(state, value.(SocketState))
		return true
	})
	data["states"] = state
	myTemplate.Execute(writer, data)
}
func WebRun() {
	go FPSCounter()
	initTemplate("server/index.html")
	http.HandleFunc("/", webHandler)
	fmt.Println("Web start...")
	err := http.ListenAndServe(":8080", nil)
	checkErr(err)
}
