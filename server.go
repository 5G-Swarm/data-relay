package main

import (
	"bytes"
	"data_relay/proto/reg_msgs"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"google.golang.org/protobuf/proto"
)

/* bind port for TCP socket
 * 'vision' data use raw image encode data protocol and relayTCPRaw function
 */
var TCPPortsDict = map[string]int{
	"reg": 10000,
	"msg": 10001,
}

// ########################################################################
// # First, get the target ID from the host address via [TCPDestDict]     #
// # Second, get the target address from the target ID via [TCPKnownList] #
// # Finally, get the conn from the target address via [TCPSockets]       #
// ########################################################################

// id to ip:port
var TCPKnownList sync.Map

// ip:port to dest id
var TCPDestDict sync.Map

// server id list
var TCPServerList sync.Map

// exist conn map
var TCPSockets sync.Map

// data_head_length
const HEAD_LENGTH = 8

// communication protocol for register information and 'message' data
type Message struct {
	Mtype string
	Pri   int
	Id    string
	Dest  string
	Data  string
}

var TCPIPs, TCPPorts sync.Map
var typeDict sync.Map // true is server, false is client

var TCPSendCnt, TCPRelayCnt, TCPRecvCnt sync.Map

var TCPStateMap sync.Map

// Block the main function
var done = make(chan struct{})

// web visualization information
type SocketState struct {
	Key      string
	Type     string
	IP       string
	Port     int
	SendFPS  int
	RelayFPS int
	RecvFPS  int
}

// index.html resource
var myTemplate *template.Template

// data for web HTML
var socketStates []SocketState

// slice is not thread-safe
var mutex sync.RWMutex

// check error, function just prints the error information
func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

// TCP step 1
func readTCP(socket *net.TCPListener, key string) {
	for {
		conn, err := socket.Accept()
		checkErr(err)
		// Store connection
		remoteAddr := conn.RemoteAddr()
		ip, _port, _ := net.SplitHostPort(remoteAddr.String())
		port, err := strconv.Atoi(_port)
		fmt.Println("Get connection. IP:", ip, "Port:", port)
		checkErr(err)
		if port == TCPPortsDict["reg"] {
			go handleReg(conn, ip, port, remoteAddr)
		} else if port == TCPPortsDict["msg"] {
			go handleMsg(conn, ip, port, remoteAddr, key)
			TCPSockets.Store(ip+":"+strconv.Itoa(port), conn)
		}
	}
}

// TCP step 2
// ##################
// #  registration  #
// ##################
func handleReg(conn net.Conn, ip string, port int, remoteAddr net.Addr) {
	defer conn.Close()
	for {
		data := make([]byte, 2048)
		reg_msg := &reg_msgs.RegInfo{}
		n, err := conn.Read(data)
		if err == io.EOF || n == 0 {
			break
		}
		proto.Unmarshal(data, reg_msg)
		TCPKnownList.Store(reg_msg.GetHostID(), ip+":"+strconv.Itoa(int(reg_msg.GetBindPort())))
		TCPDestDict.Store(ip+":"+strconv.Itoa(int(reg_msg.GetBindPort())), reg_msg.GetDestID())
		if reg_msg.GetIsServer() {
			TCPServerList.Store(reg_msg.GetHostID(), ip+":"+strconv.Itoa(int(reg_msg.GetBindPort())))
		}

		// network part

	}
}

// ##################
// #   msg_relay    #
// ##################
func handleMsg(conn net.Conn, ip string, port int, remoteAddr net.Addr, key string) {
	defer conn.Close()
	// data := []byte("")
	// data_cache := []byte("")
	// data_length := 0
	for {
		data := make([]byte, 65536)
		// conn.Read(data)
		n, err := conn.Read(data)
		if err == io.EOF || n == 0 {
			break
		}
		// if len(data_cache) != 0 {
		// 	data = binaryComb(data_cache, data)
		// 	data_cache = []byte("")
		// }
		// if data_length == 0 {
		// 	data_length = int(binary.BigEndian.Uint64(data[:HEAD_LENGTH]))
		// }
		// for {
		// 	if len(data) < data_length {
		// 		copy(data_cache, data)
		// 		break
		// 	}
		// 	send_data := data[:data_length]
		// 	data = data[data_length:]
		// go relayTCP(send_data, ip, port)

		// }
		go relayTCP(data, ip, port)
	}
}

func relayTCP(send_data []byte, ip string, port int) {
	dest_id, ok := TCPDestDict.Load(ip + ":" + strconv.Itoa(port))
	if !ok {
		fmt.Println("Destination is not registered of Host IP", ip+":"+strconv.Itoa(port))
		return
	}
	dest_ip_port, ok := TCPKnownList.Load(dest_id)
	if !ok {
		fmt.Println("Destination", dest_id, "has not been connected!")
		return
	}
	dest_conn, ok := TCPSockets.Load(dest_ip_port)
	if !ok {
		fmt.Println("Destination IP", dest_ip_port, "is disconnected!")
		return
	}
	// bi := make([]byte, 8)
	// binary.BigEndian.PutUint64(bi, uint64(len(send_data)))
	// send_data = binaryComb(bi, send_data)
	dest_conn.(net.Conn).Write(send_data)
}

// func handleTCP(conn net.Conn, ip string, port int, remoteAddr net.Addr, key string) {
// 	defer conn.Close()
// 	buffer_data := make([]byte, 0)
// 	buffer_index := 0
// 	clear_flag := false
// 	for {
// 		if clear_flag {
// 			clear_flag = false
// 			buffer_data = make([]byte, 0)
// 			buffer_index = 0
// 		}
// 		data := make([]byte, 65535*5)
// 		n, err := conn.Read(data)
// 		if err == io.EOF || n == 0 {
// 			break
// 		}
// 		var message Message
// 		/*
// 		 * error type message
// 		 */
// 		//fmt.Println("11111111111111", buffer_index)
// 		if buffer_index > 65535 {
// 			clear_flag = true
// 			continue
// 		}
// 		if err := json.Unmarshal(data[:n], &message); err != nil {
// 			//go relayTCP(data, n, remoteAddr, key)
// 			// DO NOTHING
// 			//fmt.Println("json.Unmarshal err:", err)
// 			buffer_data = append(buffer_data, data[:n]...)
// 			buffer_index = buffer_index + n
// 			//fmt.Println("DEBUG:", data[:n])
// 			//fmt.Println("buffer_index:", buffer_index)
// 			if buffer_index == 43762 || buffer_index == 10996 {
// 				fmt.Println(buffer_index, "！！！！！")
// 				if err2 := json.Unmarshal(buffer_data[:buffer_index], &message); err2 != nil {
// 					fmt.Println("WWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWW", err2)
// 					// fmt.Println("DEBUG:", buffer_data)
// 					buffer_data = make([]byte, 0)
// 					buffer_index = 0
// 					continue
// 				}
// 				//fmt.Println("333333333333333333333333")
// 				//buffer_data = make([]byte, 0)
// 				//buffer_index = 0
// 				clear_flag = true
// 			} else {
// 				continue
// 			}
// 		}
// 		checkErr(err)
// 		/*
// 		 * normal type message
// 		 */
// 		//fmt.Println("5555555555555555555")
// 		// register type message
// 		if message.Mtype == "register" {
// 			fmt.Printf("Register <%s:%s> as %s\n", key, message.Id, message.Data)
// 			// client or server ?
// 			if message.Data == "client" {
// 				typeDict.Store(message.Id, false)
// 			} else if message.Data == "server" {
// 				typeDict.Store(message.Id, true)
// 			} else {
// 				fmt.Println("Error register data", message.Data)
// 			}

// 			TCPIPs.Store(message.Id, ip)
// 			TCPPorts.Store(message.Id+":"+key, port)
// 			TCPSockets.Store(message.Id+":"+key, conn)
// 			feedback := Message{
// 				Mtype: "register",
// 				Pri:   5,
// 				Id:    "000000",
// 				Dest:  message.Id,
// 				Data:  remoteAddr.String(),
// 			}
// 			feedbackStr, err := json.Marshal(feedback)
// 			checkErr(err)
// 			_, err = conn.Write(feedbackStr)
// 			checkErr(err)
// 			// state information
// 			id := message.Id
// 			isServer, ok := typeDict.Load(id)
// 			if !ok {
// 				fmt.Println("Error !!!!")
// 				continue
// 			}
// 			stateKey := "client:" + id
// 			if isServer.(bool) {
// 				stateKey = "server:" + id
// 			}
// 			SocketState := SocketState{Key: stateKey, Type: "TCP", IP: ip, Port: port, SendFPS: 0, RelayFPS: 0, RecvFPS: 0}
// 			TCPStateMap.Store(message.Id+":"+key, SocketState)
// 			TCPSendCnt.Store(message.Id+":"+key, 0)
// 			TCPRelayCnt.Store(message.Id+":"+key, 0)
// 			TCPRecvCnt.Store(message.Id+":"+key, 0)
// 		} else { // other types message
// 			if clear_flag {
// 				go relayTCP(buffer_data, buffer_index, message, key)
// 			} else {
// 				go relayTCP(data, n, message, key)
// 			}
// 		}
// 		//fmt.Println("8888888888888888888")
// 	}
// 	// out of for, loose connection
// 	fmt.Printf("Close TCP connection <%s:%d> %s\n", ip, port, key)
// 	// try to delete key
// 	TCPIPs.Range(func(ID, otherIP interface{}) bool {
// 		// same IP
// 		if ip == otherIP {
// 			// try to get port
// 			otherPort, ok := TCPPorts.Load(ID.(string) + ":" + key)
// 			if ok {
// 				// same port
// 				if port == otherPort {
// 					TCPIPs.Delete(ID)
// 					TCPPorts.Delete(ID.(string) + ":" + key)
// 					TCPSockets.Delete(ID.(string) + ":" + key)
// 					typeDict.Delete(ID.(string))
// 					TCPSendCnt.Delete(ID.(string) + ":" + key)
// 					TCPRelayCnt.Delete(ID.(string) + ":" + key)
// 					TCPRecvCnt.Delete(ID.(string) + ":" + key)
// 					return true
// 				}
// 			}
// 		}
// 		return true
// 	})
// 	fmt.Println("Finish TCP")
// }

// TCP step 3
// func relayTCP(data []byte, n int, message Message, key string) {
// 	fmt.Println("Relay data len:", n)
// 	// state info
// 	cnt, _ := TCPSendCnt.Load(message.Id + ":" + key)
// 	// check type for safety
// 	switch cnt := cnt.(type) {
// 	case int:
// 		TCPSendCnt.Store(message.Id+":"+key, cnt+1)
// 	default:
// 		TCPSendCnt.Store(message.Id+":"+key, 0)
// 	}

// 	destID := message.Dest
// 	isServer, ok := typeDict.Load(message.Id)
// 	if !ok {
// 		fmt.Println("No ID in typeDict", message.Id)
// 		return
// 	}
// 	// server sends data to client directly
// 	if isServer.(bool) {
// 		socket, ok := TCPSockets.Load(destID + ":" + key)
// 		if !ok {
// 			fmt.Println("No destination", destID+":"+key)
// 			//fmt.Println("Data: ", message.Data)
// 			return
// 		}
// 		// send
// 		socket.(net.Conn).Write(data[:n])
// 	} else { // find a best server for client
// 		destID = findServer()
// 		if destID == "nil" {
// 			fmt.Println("No server to send data")
// 			return
// 		}
// 		socket, ok := TCPSockets.Load(destID + ":" + key)
// 		if !ok {
// 			fmt.Println("No destination", destID+":"+key)
// 			return
// 		}
// 		// send
// 		socket.(net.Conn).Write(data[:n])
// 	}

// 	// state info
// 	relayCnt, _ := TCPRelayCnt.Load(message.Id + ":" + key)
// 	// check type for safety
// 	switch relayCnt := relayCnt.(type) {
// 	case int:
// 		TCPRelayCnt.Store(message.Id+":"+key, relayCnt+1)
// 	default:
// 		TCPRelayCnt.Store(message.Id+":"+key, 0)
// 	}

// 	recvCnt, _ := TCPRecvCnt.Load(destID + ":" + key)
// 	// check type for safety
// 	switch recvCnt := recvCnt.(type) {
// 	case int:
// 		TCPRecvCnt.Store(destID+":"+key, recvCnt+1)
// 	default:
// 		TCPRecvCnt.Store(destID+":"+key, 0)
// 	}
// }

func findServer() string {
	var serverId = "nil"
	typeDict.Range(func(id, isServer interface{}) bool {
		if isServer.(bool) {
			serverId = id.(string)
		}
		return true
	})
	return serverId
}

func FPSCounter() {
	duration := time.Duration(time.Second)
	t := time.NewTicker(duration)
	defer t.Stop()
	for {
		<-t.C
		var states []SocketState
		TCPSockets.Range(func(idWithKey, socket interface{}) bool {
			sendCnt, ok := TCPSendCnt.Load(idWithKey)
			if !ok {
				fmt.Println("No key in TCPSendCnt", idWithKey)
				return true
			}
			relayCnt, ok := TCPRelayCnt.Load(idWithKey)
			if !ok {
				fmt.Println("No key in TCPRelayCnt", idWithKey)
				return true
			}
			recvCnt, ok := TCPRecvCnt.Load(idWithKey)
			if !ok {
				fmt.Println("No key in TCPRecvCnt", idWithKey)
				return true
			}
			_state, _ := TCPStateMap.Load(idWithKey)
			state := _state.(SocketState)
			state.SendFPS = sendCnt.(int)
			state.RelayFPS = relayCnt.(int)
			state.RecvFPS = recvCnt.(int)
			states = append(states, state)

			id := strings.Split(idWithKey.(string), ":")[0]
			isServer, _ := typeDict.Load(id)
			key := "client:" + id
			if isServer.(bool) {
				key = "server:" + id
			}
			newState := SocketState{Key: key, Type: "TCP", IP: state.IP, Port: state.Port, SendFPS: 0, RelayFPS: 0, RecvFPS: 0}
			TCPStateMap.Store(idWithKey, newState)
			TCPSendCnt.Store(idWithKey, 0)
			TCPRelayCnt.Store(idWithKey, 0)
			TCPRecvCnt.Store(idWithKey, 0)
			return true
		})
		mutex.Lock()
		socketStates = states
		mutex.Unlock()
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
	mutex.RLock()
	data["states"] = socketStates
	mutex.RUnlock()
	myTemplate.Execute(writer, data)
}
func binaryComb(b1, b2 []byte) (data []byte) {
	var buffer bytes.Buffer
	buffer.Write(b1)
	buffer.Write(b2)
	data = buffer.Bytes()
	return data
}

func main() {
	for key, port := range TCPPortsDict {
		clientAddr, err := net.ResolveTCPAddr("tcp4", ":"+strconv.Itoa(port))
		checkErr(err)
		clientListener, err := net.ListenTCP("tcp", clientAddr)
		checkErr(err)
		go readTCP(clientListener, key)
	}
	// go FPSCounter()
	// initTemplate("./index.html")
	// http.HandleFunc("/", webHandler)
	// err := http.ListenAndServe("0.0.0.0:8080", nil)
	// checkErr(err)
	<-done
}
