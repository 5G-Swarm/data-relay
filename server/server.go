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
var TEST_MODE = true

// bind port for TCP socket
var TCPPortsDict = map[string]int{
	"reg": 10000,
	"msg": 10001,
}

// id to ip(:port)
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
			SendFPS:  0,
			RelayFPS: 0,
			RecvFPS:  0,
		}
		TCPStateMap.Store(address, state)
		TCPSendCnt.Store(address, 0)
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

		// record successful send cnt
		sendCnt, _ := TCPSendCnt.Load(address)
		switch sendCnt := sendCnt.(type) {
		case int:
			TCPSendCnt.Store(address, sendCnt+1)
		default:
			TCPSendCnt.Store(address, 0)
		}

		// ########################################################################
		// # First, get the target ID from the host address via [TCPDestDict]     #
		// # Second, get the target address from the target ID via [TCPKnownList] #
		// # Finally, get the conn from the target address via [TCPSockets]       #
		// ########################################################################

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
	state.SendFPS = 0
	state.RelayFPS = 0
	state.RecvFPS = 0
	defer TCPStateMap.Store(address, state)
}

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
