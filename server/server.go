package server

import (
	"data_relay/proto/reg_msgs"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"google.golang.org/protobuf/proto"
)

// In test mode, we need both ip and port to distinguish between 'robot'
// and 'server'.

// bind port for TCP socket
var TCPPortsDict = map[string]int{
	"reg":  10000,
	"msg":  10001,
	"img":  10002,
	"sync": 10003,
}

var AddressMap sync.Map
var DestMap sync.Map
var TCPServerList sync.Map
var TCPSockets sync.Map
var TCPStateMap sync.Map

// data_head_length
const HEAD_LENGTH = 8

// random destination id
const RANDOM_DEST = "986677"

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
		checkErr(err)
		fmt.Println("\033[1;34m Get connection. IP:", ip, "Port:", port, "Key:", key, "\033[0m")

		var hostID = handleReg(conn, ip, port)

		TCPSockets.Store(hostID+"/"+key, conn)
		fmt.Println("SEART to relay !!!")
		go handleMsg(conn, ip, port, hostID, key)
	}
}

// TCP step 2
// ##################
// #  registration  #
// ##################
func handleReg(conn net.Conn, ip string, port int) string {
	data := make([]byte, 2048)
	reg_msg := &reg_msgs.RegInfo{}
	n, err := conn.Read(data)
	if err == io.EOF || n == 0 {
		fmt.Println("Read data error in func: [handleReg]")
		return ""
	}
	proto.Unmarshal(data, reg_msg)
	var reg_key = string(reg_msg.GetKey())
	var hostID = string(reg_msg.GetHostID())
	var destID = string(reg_msg.GetDestID())
	var isServer = bool(reg_msg.GetIsServer())

	var address = ip + ":" + strconv.Itoa(port)
	var idKey = hostID + "/" + reg_key

	fmt.Println("[DEBUG] reg address", address, port)
	AddressMap.Store(idKey, address)
	DestMap.Store(idKey, destID+"/"+reg_key)

	if isServer {
		TCPServerList.Store(idKey, address)
	}

	// network part
	state := SocketState{
		ID:       idKey,
		Dest:     destID,
		State:    "Connected",
		IsServer: isServer,
		IP:       address,
		SendFPS:  0,
		RelayFPS: 0,
		RecvFPS:  0,
	}
	TCPStateMap.Store(idKey, state)
	TCPSendCnt.Store(idKey, 0)
	TCPRelayCnt.Store(idKey, 0)
	TCPRecvCnt.Store(idKey, 0)

	fmt.Println("\033[1;32m Establish connection: HostID:", reg_msg.GetHostID(), "Key:", reg_key, "\033[0m")
	return hostID
}

// ##################
// #   msg_relay    #
// ##################
func handleMsg(conn net.Conn, ip string, port int, hostID string, key string) {
	var idKey = hostID + "/" + key
	var address = ip + ":" + strconv.Itoa(port)

	defer conn.Close()
	defer TCPSockets.Delete(idKey)
	// defer TCPServerList.Delete(address)
	defer fmt.Println("Connection closed. IP:", ip, "Port:", port)

	for {
		data := make([]byte, 65535)
		n, err := conn.Read(data)
		data = data[:n]
		if err == io.EOF || n == 0 {
			break
		}

		// record successful send cnt
		sendCnt, _ := TCPSendCnt.Load(idKey)
		switch sendCnt := sendCnt.(type) {
		case int:
			TCPSendCnt.Store(idKey, sendCnt+1)
		default:
			TCPSendCnt.Store(idKey, 0)
		}

		destIdKey, _ := DestMap.Load(idKey)
		dest_conn, ok := TCPSockets.Load(destIdKey)
		if !ok {
			fmt.Println("Destination ", destIdKey, "is disconnected!")
			continue
		}

		// record successful relay cnt
		relayCnt, _ := TCPRelayCnt.Load(idKey)
		switch relayCnt := relayCnt.(type) {
		case int:
			TCPRelayCnt.Store(idKey, relayCnt+1)
		default:
			TCPRelayCnt.Store(idKey, 0)
		}

		dest_conn.(net.Conn).Write(data)

		// record successful recv cnt of destination
		recvCnt, _ := TCPRecvCnt.Load(destIdKey)
		switch recvCnt := recvCnt.(type) {
		case int:
			TCPRecvCnt.Store(destIdKey, recvCnt+1)
		default:
			TCPRecvCnt.Store(destIdKey, 0)
		}
	}
	// change the connection state
	var _state, _ = TCPStateMap.Load(address)
	var state = _state.(SocketState)
	state.State = "Disconnected"
	state.SendFPS = 0
	state.RelayFPS = 0
	state.RecvFPS = 0
	TCPStateMap.Store(address, state)
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
