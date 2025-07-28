package main

import (
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var Version = "(untracked)"

const (
	// 2097151 is the max len that fit in 3 byte varint
	PACKET_SIZE = 2097151 + 3

	// packet size + 3 byte info
	// 1 byte for protocol / 2 byte for client id
	// not the most effient way ikr
	MSG_SIZE = 2097154 + 3

	OUT_ADDR_STR = "0.0.0.0:19654"
	// UDP_ADDR_STR = "0.0.0.0:19132"
	// TCP_ADDR_STR = "0.0.0.0:25565"
	UDP_ADDR_STR = "0.0.0.0:19655"
	TCP_ADDR_STR = "0.0.0.0:19656"

	// very secret dont commit this to git!!!!
	OUTSIDE_SECRET     = "4ZN9GZU8LBIIYZ76HJMQLKJGZ52RULK2PFVYK64HYGX75UNHLX9FY2SHPX5WWL8I"
	OUTSIDE_SECRET_LEN = 64

	MAX_CLIENT_SIZE = 64
	TCP_CLIENT_SIZE = 32
	UDP_CLIENT_SIZE = 32
)

func main() {
	log.Println("version:", Version)

	if MAX_CLIENT_SIZE != TCP_CLIENT_SIZE+UDP_CLIENT_SIZE { // ASSERTION
		log.Fatal("TCP_CLIENT_SIZE + UDP_CLIENT_SIZE don't add up to MAX_CLIENT_SIZE")
		return
	}

	done := make(chan bool, 1)

	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
		<-sigch

		done <- true
	}()

	go func() {

		Outside()

		log.Println("Outside ended")

		done <- true
	}()

	<-done

}

func Outside() {
	log.Println("Outside started")

	var err error

	var listener *net.TCPListener = nil
	var listener_addr *net.TCPAddr = nil

	var accept_loop = 0
	var inside net.Conn = nil

	var inside_client_tcp_count = 0
	var inside_client_tcp_list []net.Conn = nil

	//
	// Listener start Phase
	//
	for {
		listener_addr, err = net.ResolveTCPAddr("tcp", OUT_ADDR_STR)
		if err != nil {
			log.Println("Outside ResolveTCPAddr failed , err: ", err.Error())
			continue
		}

		listener, err = net.ListenTCP("tcp", listener_addr)
		if err != nil {
			log.Println("Outside ListenTCP failed , err: ", err.Error())
			continue
		}

		break
	}

	if listener == nil { // ASSERTION
		log.Fatal("Outside Listener SHOULD NOT be nil")
		return
	}
	if listener_addr == nil { // ASSERTION
		log.Fatal("Outside ListenerAddr SHOULD NOT be nil")
		return
	}

	for {
		accept_loop += 1
		inside_client_tcp_count = 0
		log.Println("Outside accept inside loop ", accept_loop, " started")

		//
		// accept phase
		//
		inside, err = listener.Accept()
		if err != nil {
			log.Println("Outside accept new inside failed, err: ", err.Error())
			continue
		}
		if inside == nil { // ASSERTION
			log.Fatal("Inside net.Conn SHOULD NOT be nil")
			return
		}

		//
		// Read OUTSIDE_SECRET phase
		//
		read_buffer := make([]byte, OUTSIDE_SECRET_LEN)
		read, err := inside.Read(read_buffer)
		if err != nil {
			log.Println("Outside read from inside failed , err: ", err.Error())
			continue
		}
		if read == 0 {
			continue
		}
		if read != OUTSIDE_SECRET_LEN {
			log.Println("Outside couldnt read ", OUTSIDE_SECRET_LEN, " bytes from inside")
			continue
		}

		if string(read_buffer) == OUTSIDE_SECRET {
			log.Println("Outside read OUTSIDE_SECRET from inside")
		} else {
			log.Println("Outside couldnt read OUTSIDE_SECRET from inside")
			continue
		}

		//
		// Write OK phase
		//
		// ALERT: THIS CAN FAIL USE A WRITE ALL FUNCTION!!!!
		write, err := inside.Write([]byte("OK"))
		if err != nil {
			log.Println("Outside write to inside failed , err: ", err.Error())
			continue
		}
		if write == 0 {
			continue
		}
		if write != 2 {
			log.Println("Outside couldnt write 2 bytes to inside")
			continue
		}

		//
		// Read OK phase
		//
		read_buffer = make([]byte, 2)
		read, err = inside.Read(read_buffer)
		if err != nil {
			log.Println("Outside read from inside failed , err: ", err.Error())
			continue
		}
		if read == 0 {
			continue
		}
		if read != 2 {
			log.Println("Outside couldnt read 2 bytes from inside")
			continue
		}

		if string(read_buffer) == "OK" {
			log.Println("Outside read OK from inside")
		} else {
			log.Println("Outside couldnt read OK from inside")
			continue
		}

		//
		// Accept inside clients phase
		//
		inside_client_tcp_list = make([]net.Conn, TCP_CLIENT_SIZE)
		for { // ALERT: INFINITE LOOP CAN HAPPEN

			client, err := listener.Accept()
			if err != nil {
				log.Println(err)
				continue
			}

			inside_remote_addr := strings.Split(client.RemoteAddr().String(), ":")[0]
			client_remote_addr := strings.Split(inside.RemoteAddr().String(), ":")[0]

			if inside_remote_addr == client_remote_addr {

				inside_client_tcp_list[inside_client_tcp_count] = client
				inside_client_tcp_count += 1

			}

			// TODO: READ AND WRITE OK FROM INSIDE/OUTSIDE for each client?

			if inside_client_tcp_count == TCP_CLIENT_SIZE {
				log.Println("Outside connected to", TCP_CLIENT_SIZE, " inside clients")
				break
			}

		}

		// TODO: find a way to assert this
		if inside_client_tcp_list == nil { // ASSERTION
			log.Fatal("Inside tcp client list SHOULD NOT be nil")
			return
		}

		if inside_client_tcp_count != TCP_CLIENT_SIZE { // ASSERTION
			log.Fatal("Inside tcp client count SHOULD Be ", TCP_CLIENT_SIZE, " instead of ", inside_client_tcp_count)
			return
		}

		//
		// Prevent From Accepting New Clients
		//
		// TODO: MAYBE remove the whole goroutine? should behave the same
		done := make(chan bool, 1)
		go func() {

			Tunnel(listener, inside, inside_client_tcp_list)

			log.Println("Tunnel ended")

			done <- true

		}()
		<-done

		log.Println("Outside accept inside loop ", accept_loop, "ended")
	}

}

func Tunnel(outside *net.TCPListener, inside net.Conn, ictl []net.Conn) {
	log.Println("Tunnel Started")

	var err error

	var tcp_addr *net.TCPAddr = nil
	var tcp_listener *net.TCPListener = nil

	client_tcp_count := 0
	client_tcp_list := make([]net.Conn, TCP_CLIENT_SIZE)
	client_tcp_occupied := make([]bool, TCP_CLIENT_SIZE)

	// TODO: break it into 2 for loops to handle error
	// TODO: add maxium retries per each for loop -> prevents infinite looping
	for { // ALERT: INFINITE LOOP CAN HAPPEN
		tcp_addr, err = net.ResolveTCPAddr("tcp", TCP_ADDR_STR)
		if err != nil {
			log.Println("Outside ResolveTCPAddr failed , err:", err.Error())
			continue
		}

		tcp_listener, err = net.ListenTCP("tcp", tcp_addr)
		if err != nil {
			log.Println("Outside ListenTCP failed , err:", err.Error())
			continue
		}

		break
	}

	if tcp_addr == nil { // ASSERTION
		log.Fatal("Outside tcp_addr SHOULD NOT be nil")
		return
	}
	if tcp_listener == nil { // ASSERTION
		log.Fatal("Outside tcp_listener SHOULD NOT be nil")
		return
	}

	done := make(chan bool, 1)

	go func() {
		PingInside(inside)

		inside.Close()
		tcp_listener.Close()
		// TODO: Close inside/client Connections

		log.Println("PingInside ended")

		done <- true
	}()

	go func() {
		for {
			// TODO: MAYBE Less CPU?
			if client_tcp_count == TCP_CLIENT_SIZE {
				time.Sleep(time.Second * 1)

				continue
			}

			client, err := tcp_listener.Accept()
			if err != nil {
				// log.Print(err)
				continue
			}

			client_id, err := first_occupied(client_tcp_occupied)
			if err != nil {
				log.Fatalln("list shouldnt be full but it is ,err:", err.Error())
				return
			}
			println(client_id)

			client_tcp_count += 1
			client_tcp_list[client_id] = client
			client_tcp_occupied[client_id] = true

			go ListenClient(client_tcp_list[client_id], ictl[client_id], client_id, &client_tcp_count, client_tcp_occupied)
			go ListenInside(client_tcp_list[client_id], ictl[client_id], client_id, &client_tcp_count, client_tcp_occupied)

		}
	}()

	<-done

}

func PingInside(inside net.Conn) {
	var time_freez time.Time = time.Now()

	log.Println("PingInside started")

	for {
		//
		// CHECK TIMEOUT PHASE
		//

		// Check if last read+write happened withing 2 seconds
		if time.Since(time_freez).Seconds() > 2 {
			return
		}

		//
		// Write 0 phase
		//

		write, err := inside.Write([]byte("0"))
		if errors.Is(err, syscall.EPIPE) {
			log.Println("PingInside EPIPE")
			return
		}
		if err != nil {
			// log.Println("PingInside write to inside failed , err: ", io.ErrClosedPipe.Error())
			// log.Println("PingInside write to inside failed , err: ", syscall.ESPIPE)
			// log.Println("PingInside write to inside failed , err: ", syscall.EPIPE)
			log.Println("PingInside write to inside failed , err: ", err.Error())
			continue
		}
		if write == 0 {
			continue
		}

		//
		// Read 1 phase
		//

		read_buffer := make([]byte, 1)
		read, err := inside.Read(read_buffer)
		if err != nil {
			log.Println("PingInside read from inside failed , err: ", err.Error())
			continue
		}
		if read == 0 {
			continue
		}

		if string(read_buffer) == "1" {
			// log.Println("PingInside read 1 from inside")
		} else {
			log.Println("PingInside couldnt read 1 from inside")
			continue
		}

		time_freez = time.Now()
		time.Sleep(time.Millisecond * 250)
	}
}

func ListenClient(client net.Conn, inside net.Conn, client_id int, ctc *int, cto []bool) {
	// TODO: ADD TIMEOUT
	// TODO: CHECK AND REMOVE CLIENT

	//
	// Alert Inside client that a new connection was made
	//
	start_buffer := make([]byte, 1)
	copy(start_buffer, []byte("0"))

	write, err := WriteAll(inside, start_buffer, len(start_buffer))
	if err != nil {
		log.Println("ListenClient ", client_id, " write all to inside failed , err: ", err.Error())
		cto[client_id] = false
		defer client.Close()
		return
	}
	if write != 1 {
		log.Println("ListenClient ", client_id, " write all to inside failed")
		return
	}

	//
	// Read from client -> Send to inside PHASE
	//
	eof := 0
	for {
		if eof > 10 {
			return
		}

		read_buffer := make([]byte, PACKET_SIZE)
		read, err := client.Read(read_buffer)

		if errors.Is(err, io.EOF) {
			eof += 1
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if err != nil {
			log.Println("ListenClient ", client_id, " read from client failed , err:", err.Error())
			continue
		}
		if read == 0 {
			continue
		}

		write_buffer := make([]byte, read)
		copy(write_buffer, read_buffer[:read])

		write, err := WriteAll(inside, write_buffer, read)

		if errors.Is(err, syscall.EPIPE) {
			//
			// DEFER CLIENT
			//
			*ctc -= 1
			client.Close()
			cto[client_id] = false

			log.Println()
			log.Println()
			log.Println()
			log.Println()
			log.Println()
			log.Println()
			log.Println()
			log.Println()

			break
		}

		if err != nil {
			log.Println("ListenClient ", client_id, " write all to inside failed , err:", err.Error())
			continue
		}
		if write != 1 {
			continue
		}
	}

}

func ListenInside(client net.Conn, inside net.Conn, client_id int, ctc *int, cto []bool) {

	//
	// Read from inside -> Send to client PHASE
	//
	eof := 0
	for {
		if eof > 10 {
			return
		}

		read_buffer := make([]byte, PACKET_SIZE)
		read, err := inside.Read(read_buffer)

		if errors.Is(err, io.EOF) {
			eof += 1
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if err != nil {
			log.Println("ListenClient ", client_id, " read from inside failed , err:", err.Error())
			continue
		}
		if read == 0 {
			continue
		}

		write_buffer := make([]byte, read)
		copy(write_buffer, read_buffer[:read])
		write, err := WriteAll(client, write_buffer, read)
		if errors.Is(err, syscall.EPIPE) {
			//
			// BREAK HAPPENS IN ListenClient
			//

			break
		}
		if err != nil {
			log.Println("ListenClient ", client_id, " write all to client failed , err:", err.Error())
			continue
		}
		if write != 1 {
			continue
		}
	}
}

func first_occupied(list []bool) (int, error) {
	for i, v := range list {
		if !v {
			return i, nil
		}
		continue
	}
	return 0, errors.New("no empty slots in list ")
}

func ReadVarInt(buf []byte) (len int, out int) {
	const extend = 0b1000_0000  // 128
	const exclude = 0b0111_1111 // 127

	out = 0
	len = 0
	for {
		out = out | ((int(buf[len]) & exclude) << (len * 7))
		if (int(buf[len]) & extend) == extend {
			len += 1
		} else {
			return len, out
		}
	}

}

// TODO: Handle infinite err loop
// MAYBE return on err?
//
// TODO: Handle when readall reads exceed buf/len size
func ReadAll(conn net.Conn, buf []byte, len int) (int, error) {
	var i = 0
	for i < len { // ALERT: INFINITE LOOP CAN HAPPEN
		read, err := conn.Read(buf)
		if err != nil {
			log.Println("ReadAll failed , err", err)
			continue
		}

		i = i + read
	}

	return i, nil
}

// TODO: Handle infinite err loop
func WriteAll(conn net.Conn, buf []byte, len int) (int, error) {
	var i = 0
	for i < len { // ALERT: INFINITE LOOP CAN HAPPEN
		write, err := conn.Write(buf[i:])

		//
		// TODO: THIS DOESNT WORK FIX IT!!
		//
		if errors.Is(err, syscall.EPIPE) {
			log.Println("WriteAll failed ,broken pipe err:", err.Error())
			return 0, err
		}

		if err != nil {
			log.Println("WriteAll failed , err", err.Error())
			continue
		}

		i = i + write
	}

	return i, nil
}

// INFINITE LOOP NOTICE!!!!
func WriteTcpClient(client net.Conn, client_id int, msg_len int, msg []byte) {

	var i = 0
	for i < msg_len {
		write, err := client.Write(msg)
		if err != nil {
			log.Print(err)
			continue
		}

		i = i + write

		if write == 0 {
			panic("WRITE PANIC")
		}
	}

	log.Println(client_id, "client write ", i)

	if msg_len <= 16 {
		log.Println(client_id, "client write buffer ", msg[:msg_len])
	} else {
		log.Println(client_id, "client write buffer ", msg[:16])
	}
}

// INFINITE LOOP NOTICE!!!!
func WriteInside(inside net.Conn, client_id int, msg_len int, msg []byte) {

	var i = 0
	for i < msg_len {

		write_buffer := make([]byte, (msg_len-i)+3)
		copy(write_buffer[3:], msg[i:msg_len])

		// tcp
		write_buffer[0] = 1

		// TODO: Handle when client count is > 255
		write_buffer[1] = byte(0)
		write_buffer[2] = byte(client_id)

		write, err := inside.Write(write_buffer)
		if err != nil {
			log.Print(err)
			continue
		}

		if write >= 3 {
			i = i + write - 3
		}

		if write == 0 {
			panic("WTF! A WRITE PANIC ?")
		}
	}

	log.Println(client_id, "inside write ", i)

	if msg_len <= 16 {
		log.Println(client_id, "inside write buffer ", msg[:msg_len])
	} else {
		log.Println(client_id, "inside write buffer ", msg[:16])
	}

}
