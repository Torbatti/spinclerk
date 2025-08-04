package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

const (
	PACKET_SIZE = 2097154
)

var Version = "(untracked)"

func main() {

	udp_addr_string := "0.0.0.0:19653"
	tcp_addr_string := "0.0.0.0:19652"

	// TODO(AABI): Rework cli
	// --udp=0.0.0.0:19653
	// --tcp=0.0.0.0:19652
	args := os.Args
	for i, v := range args {
		switch v {
		case "udp":
			// todo: this can break via adding anything but an address
			udp_addr_string = args[i+1]
		case "tcp":
			// todo: this can break via adding anything but an address
			tcp_addr_string = args[i+1]

		}
	}

	done := make(chan bool, 1)

	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
		<-sigch

		done <- true
	}()

	go func() {
		ListenTcp(tcp_addr_string)

		done <- true
	}()

	go func() {
		ListenUdp(udp_addr_string)

		done <- true
	}()

	<-done

}

func ListenTcp(addr_string string) {
	addr, err := net.ResolveTCPAddr("tcp", addr_string)
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	client_count := 0

	for {
		client, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
			continue
		}

		target, err := net.Dial("tcp", "localhost:25565")
		if err != nil {
			log.Fatal(err)
			continue
		}

		go HandleRecvFromClientTCP(client, target, client_count)
		go HandleRecvFromTargetTCP(client, target, client_count)
	}
}

func HandleRecvFromClientTCP(client net.Conn, target net.Conn, client_id int) {

	defer client.Close()
	defer target.Close()

	for {
		read_buffer := make([]byte, PACKET_SIZE)

		read, err := client.Read(read_buffer)
		if err != nil {
			// log.Print(err)
			continue
		}

		if read == 0 {
			continue
		}

		write_buffer := make([]byte, read)
		copy(write_buffer, read_buffer[:read])

		var i = 0
		for i < read {
			write, err := target.Write(write_buffer[i:])
			if err != nil {
				// log.Print(err)
				continue
			}

			i = i + write

			if write == 0 {
				continue
			}
		}
	}
}

func HandleRecvFromTargetTCP(client net.Conn, target net.Conn, client_id int) {

	defer client.Close()
	defer target.Close()

	for {
		read_buffer := make([]byte, PACKET_SIZE)

		read, err := target.Read(read_buffer)
		if err != nil {
			// log.Print(err)
			continue
		}

		if read == 0 {
			continue
		}

		write_buffer := make([]byte, read)
		copy(write_buffer, read_buffer[:read])

		var i = 0
		for i < read {
			write, err := client.Write(write_buffer[i:])
			if err != nil {
				// log.Print(err)
				continue
			}

			i = i + write

			if write == 0 {
				continue
			}
		}

	}
}

func ListenUdp(addr_string string) {
	addr, err := net.ResolveUDPAddr("udp", addr_string)
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	addr_list := make([]net.Addr, 1024)
	client_list := make([]net.Conn, 1024)
	client_count := 0

	for {
		read_buffer := make([]byte, PACKET_SIZE)

		read, addr, err := listener.ReadFromUDP(read_buffer)
		if err != nil {
			log.Print(err)
			continue
		}

		connected_addr := AddrConnected(addr_list, addr, client_count)

		if connected_addr == -1 {

			target, err := net.DialUDP("udp", nil, &net.UDPAddr{
				IP:   net.ParseIP("0.0.0.0"),
				Port: 19132,
			})
			if err != nil {
				log.Fatal(err)
			}

			go HandleRecvFromTargetUDP(listener, addr, target, client_count)

			addr_list[client_count] = addr
			client_list[client_count] = target
			client_count += 1

			if read == 0 {
				continue
			}

			write_buffer := make([]byte, read)
			copy(write_buffer, read_buffer[:read])

			var i = 0
			for i < read {
				write, err := target.Write(write_buffer[i:])
				if err != nil {
					// log.Print(err)
					continue
				}

				i = i + write

				if write == 0 {
					continue
				}
			}
		} else {

			if read == 0 {
				continue
			}

			write_buffer := make([]byte, read)
			copy(write_buffer, read_buffer[:read])

			var i = 0
			for i < read {
				write, err := client_list[connected_addr].Write(write_buffer[i:])
				if err != nil {
					// log.Print(err)
					continue
				}

				i = i + write

				if write == 0 {
					continue
				}
			}
		}
	}
}

func AddrConnected(addr_list []net.Addr, addr net.Addr, client_count int) int {
	for i, v := range addr_list[:client_count] {
		if v.String() == addr.String() {
			return i
		}
	}

	return -1
}

func HandleRecvFromTargetUDP(listener *net.UDPConn, client net.Addr, target net.Conn, client_id int) {

	for {
		read_buffer := make([]byte, PACKET_SIZE)

		read, err := target.Read(read_buffer)
		if err != nil {
			// log.Print(err)
			continue
		}

		if read == 0 {
			continue
		}

		write_buffer := make([]byte, read)
		copy(write_buffer, read_buffer[:read])

		var i = 0
		for i < read {
			write, err := listener.WriteTo(write_buffer[i:], client)
			if err != nil {
				// log.Print(err)
				continue
			}

			i = i + write

			if write == 0 {
				continue
			}
		}

	}
}
