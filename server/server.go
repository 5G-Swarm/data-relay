package server

import (
	"data_relay/proto/reg_msgs"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"sync"

	"google.golang.org/protobuf/proto"
)

// In test mode, we need both ip and port to distinguish between 'robot'
// and 'server'.
var TEST_MODE = false

// bind port for TCP socket
// 'vision' data use raw image encode data protocol and relayTCPRaw function
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

// random destination id
const RANDOM_DEST = "986677"

// communication protocol for register information and 'message' data
type Message struct {
	Mtype string
	Pri   int
	Id    string
	Dest  string
	Data  string
}

var TCPStateMap sync.Map

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
		if key == "reg" {
			go handleReg(conn, ip, port, remoteAddr)
		} else if key == "msg" {
			go handleMsg(conn, ip, port, remoteAddr, key)
			if TEST_MODE {
				TCPSockets.Store(ip+":"+strconv.Itoa(port), conn)
			} else {
				TCPSockets.Store(ip, conn)
			}
		}
	}
}

// TCP step 2
// ##################
// #  registration  #
// ##################
func handleReg(conn net.Conn, ip string, port int, remoteAddr net.Addr) {
	defer conn.Close()
	defer fmt.Println("Connection closed. IP:", ip, "Port:", port)
	for {
		data := make([]byte, 2048)
		reg_msg := &reg_msgs.RegInfo{}
		n, err := conn.Read(data)
		if err == io.EOF || n == 0 {
			break
		}
		proto.Unmarshal(data, reg_msg)
		var address = ip
		if TEST_MODE {
			address = ip + ":" + strconv.Itoa(int(reg_msg.GetBindPort()))
		}
		TCPKnownList.Store(reg_msg.GetHostID(), address)

		// choose an avalible server randomly for host robot
		if reg_msg.GetDestID() == RANDOM_DEST {
			var mapKeys []string
			TCPServerList.Range(func(key, value interface{}) bool {
				mapKeys = append(mapKeys, key.(string))
				return true
			})
			TCPDestDict.Store(address, mapKeys[rand.Intn(len(mapKeys))])
		} else {
			TCPDestDict.Store(address, reg_msg.GetDestID())
		}

		if reg_msg.GetIsServer() {
			TCPServerList.Store(reg_msg.GetHostID(), address)
		}

		// network part
		state := SocketState{
			ID:       reg_msg.GetHostID(),
			Dest:     reg_msg.GetDestID(),
			State:    "Connected",
			IsServer: reg_msg.GetIsServer(),
			IP:       address,
			RelayFPS: 0,
			RelayFPS: 0,
			RecvFPS:  0,
		}
		TCPStateMap.Store(address, state)
		TCPRelayCnt.Store(address, 0)
		TCPRelayCnt.Store(address, 0)
		TCPRecvCnt.Store(address, 0)
	}
}

// ##################
// #   msg_relay    #
// ##################
func handleMsg(conn net.Conn, ip string, port int, remoteAddr net.Addr, key string) {
	var address = ip
	if TEST_MODE {
		address = ip + ":" + strconv.Itoa(port)
	}
	defer conn.Close()
	defer TCPSockets.Delete(address)
	defer TCPServerList.Delete(address)
	defer fmt.Println("Connection closed. IP:", ip, "Port:", port)
	for {
		data := make([]byte, 4096)
		// conn.Read(data)
		n, err := conn.Read(data)

		// delete empty data
		data = data[:n]

		if err == io.EOF || n == 0 {
			break
		}

		// record successful Relay cnt
		RelayCnt, _ := TCPRelayCnt.Load(address)
		switch RelayCnt := RelayCnt.(type) {
		case int:
			TCPRelayCnt.Store(address, RelayCnt+1)
		default:
			TCPRelayCnt.Store(address, 0)
		}

		dest_id, ok := TCPDestDict.Load(address)
		if !ok {
			fmt.Println("Destination of Host ", ip+":"+strconv.Itoa(port), " is not registered.")
			return
		}
		dest_address, ok := TCPKnownList.Load(dest_id)
		if !ok {
			fmt.Println("Destination", dest_id, "has not been connected!")
			return
		}
		dest_conn, ok := TCPSockets.Load(dest_address)
		if !ok {
			fmt.Println("Destination IP", dest_address, "is disconnected!")
			return
		}

		// record successful relay cnt
		relayCnt, _ := TCPRelayCnt.Load(address)
		switch relayCnt := relayCnt.(type) {
		case int:
			TCPRelayCnt.Store(address, relayCnt+1)
		default:
			TCPRelayCnt.Store(address, 0)
		}

		dest_conn.(net.Conn).Write(data)

		// record successful recv cnt of destination
		recvCnt, _ := TCPRecvCnt.Load(dest_address)
		switch recvCnt := recvCnt.(type) {
		case int:
			TCPRecvCnt.Store(address, recvCnt+1)
		default:
			TCPRecvCnt.Store(address, 0)
		}
	}
	// change the connection state
	var _state, _ = TCPStateMap.Load(address)
	var state = _state.(SocketState)
	state.State = "Disconnected"
	state.RelayFPS = 0
	state.RelayFPS = 0
	state.RecvFPS = 0
	defer TCPStateMap.Store(address, state)
}

// func relayTCP(Relay_data []byte, ip string, port int) {
// 	dest_id, ok := TCPDestDict.Load(ip + ":" + strconv.Itoa(port))
// 	if !ok {
// 		fmt.Println("Destination is not registered of Host IP", ip+":"+strconv.Itoa(port))
// 		return
// 	}
// 	dest_ip_port, ok := TCPKnownList.Load(dest_id)
// 	if !ok {
// 		fmt.Println("Destination", dest_id, "has not been connected!")
// 		return
// 	}
// 	dest_conn, ok := TCPSockets.Load(dest_ip_port)
// 	if !ok {
// 		fmt.Println("Destination IP", dest_ip_port, "is disconnected!")
// 		return
// 	}
// 	// bi := make([]byte, 8)
// 	// binary.BigEndian.PutUint64(bi, uint64(len(Relay_data)))
// 	// Relay_data = binaryComb(bi, Relay_data)
// 	n, err := dest_conn.(net.Conn).Write(Relay_data)
// 	if err == io.EOF || n == 0 {
// 		fmt.Println("Invalid Relay!")
// 	}
// }

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
// 			SocketState := SocketState{Key: stateKey, Type: "TCP", IP: ip, Port: port, RelayFPS: 0, RelayFPS: 0, RecvFPS: 0}
// 			TCPStateMap.Store(message.Id+":"+key, SocketState)
// 			TCPRelayCnt.Store(message.Id+":"+key, 0)
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
// 					TCPRelayCnt.Delete(ID.(string) + ":" + key)
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
// 	cnt, _ := TCPRelayCnt.Load(message.Id + ":" + key)
// 	// check type for safety
// 	switch cnt := cnt.(type) {
// 	case int:
// 		TCPRelayCnt.Store(message.Id+":"+key, cnt+1)
// 	default:
// 		TCPRelayCnt.Store(message.Id+":"+key, 0)
// 	}

// 	destID := message.Dest
// 	isServer, ok := typeDict.Load(message.Id)
// 	if !ok {
// 		fmt.Println("No ID in typeDict", message.Id)
// 		return
// 	}
// 	// server Relays data to client directly
// 	if isServer.(bool) {
// 		socket, ok := TCPSockets.Load(destID + ":" + key)
// 		if !ok {
// 			fmt.Println("No destination", destID+":"+key)
// 			//fmt.Println("Data: ", message.Data)
// 			return
// 		}
// 		// Relay
// 		socket.(net.Conn).Write(data[:n])
// 	} else { // find a best server for client
// 		destID = findServer()
// 		if destID == "nil" {
// 			fmt.Println("No server to Relay data")
// 			return
// 		}
// 		socket, ok := TCPSockets.Load(destID + ":" + key)
// 		if !ok {
// 			fmt.Println("No destination", destID+":"+key)
// 			return
// 		}
// 		// Relay
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

// func binaryComb(b1, b2 []byte) (data []byte) {
// 	var buffer bytes.Buffer
// 	buffer.Write(b1)
// 	buffer.Write(b2)
// 	data = buffer.Bytes()
// 	return data
// }

func Run() {
	fmt.Println("Server start...")
	for key, port := range TCPPortsDict {
		clientAddr, err := net.ResolveTCPAddr("tcp4", ":"+strconv.Itoa(port))
		checkErr(err)
		clientListener, err := net.ListenTCP("tcp", clientAddr)
		checkErr(err)
		go readTCP(clientListener, key)
	}
	WebRun()
}
