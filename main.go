package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
)

var users = make(map[net.Conn]string)
var messageHistory []string
var mu sync.Mutex
var file *os.File
var shuttingDown = false

const pinguin = "\x1b[31m" + `Welcome to TCP-Chat!
         _nnnn_
        dGGGGMMb
       @p~qp~~qMb
       M|@||@) M|
       @,----.JM|
      JS^\__/  qKL
     dZP        qKRb
    dZP          qKKb
   fZP            SMMb
   HZM            MMMM
   FqM            MMMM
 __| ".        |\dS"qML
 |    '.       | '' \Zq
_)      \.___.,|     .'
\____   )MMMMMP|   .'
     '-'       '--'
` + "\x1b[0m"

func main() {
	port := "8080"
	host := "localhost"
	if len(os.Args) > 2 {
		host = os.Args[1]
		port = os.Args[2]
		_, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Println("[USAGE]: ./TCPChat $port")
			return
		}
	} else if len(os.Args) == 2 {
		port = os.Args[1]
	}
	ln, err := net.Listen("tcp4", host+":"+port)
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}
	startMessage := fmt.Sprintf("[%s] Server started on %s:%s\n", getTimeFormatted(), host, port)
	writeFile(startMessage)
	fmt.Print(startMessage)

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)
	go func() {
		<-stopChan
		shuttingDown = true
		stopMessage := fmt.Sprintf("[%s] Server stopped\n", getTimeFormatted())
		writeFile(stopMessage)
		fmt.Print(stopMessage)
		ln.Close()
		os.Exit(0)
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if shuttingDown {
				break
			}
			fmt.Println("Error accepting:", err)
			continue
		}
		if len(users) >= 10 {
			conn.Write([]byte("Server is full. Try again later\n"))
			conn.Close()
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	conn.Write([]byte(pinguin))
	askUsername(conn)
	logMessage := fmt.Sprintf("[%s] New user connected : %s. Username : %s\n", getTimeFormatted(), conn.LocalAddr(), users[conn])
	fmt.Print(logMessage)
	writeFile(logMessage)
	sendHistory(conn)
	broadcastMessage(fmt.Sprintf("[%s] %s joined the chat\n", getTimeFormatted(), users[conn]))
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				disconnectMessage := "[" + fmt.Sprintf(getTimeFormatted()+"] "+users[conn]+" has been disconnected\n")
				fmt.Print(disconnectMessage)
				writeFile(disconnectMessage)
				break
			}
			fmt.Println("Error reading:", err)
			break
		}
		if string(buf[:n]) == "/exit\n" {
			break
		}
		if string(buf[:n]) == "/rename\n" {
			lastName := users[conn]
			askUsername(conn)
			renameMessage := fmt.Sprintf("[%s] %s changed his name to %s\n", getTimeFormatted(), lastName, users[conn])
			fmt.Print(renameMessage)
			broadcastMessage(renameMessage)
			continue
		}
		if string(buf[:n]) != "\n" {
			message := fmt.Sprintf("[%s] %s: %s", getTimeFormatted(), users[conn], string(buf[:n]))
			fmt.Print(message)
			messageHistory = append(messageHistory, message)
			broadcastMessage(message)
		}
	}
	defer func() {
		broadcastMessage(fmt.Sprintf("[%s] %s left the chat\n", getTimeFormatted(), users[conn]))
		delete(users, conn)
		conn.Close()
	}()
}

func askUsername(conn net.Conn) {
	conn.Write([]byte("Enter your username: "))
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		username := scanner.Text()
		mu.Lock()
		users[conn] = username
		mu.Unlock()
	}
	if users[conn] == "" {
		users[conn] = "Anonymous"
	}
}

func sendHistory(conn net.Conn) {
	for _, message := range messageHistory {
		conn.Write([]byte(message))
	}
}
func broadcastMessage(message string) {
	mu.Lock()
	for conn := range users {
		_, err := conn.Write([]byte(message))
		if err != nil {
			fmt.Println("Error writing to connection:", err)
		}
	}
	writeFile(message)
	mu.Unlock()
}
func getTimeFormatted() string {
	return time.Now().Format("02/01/2006 - 15:04:05")
}

func writeFile(message string) {
	var err error
	file, err = os.OpenFile("./logs/log"+time.Now().Format("02012006")+".txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
	}
	defer file.Close()
	if _, err := file.WriteString(message); err != nil {
		fmt.Println("Error writing to file:", err)
	}
}
