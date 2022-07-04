/*
* @Author: Chris Burch
* @Date: 2022-4-4
* @Description: Xcloud extension listener.
 */
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"unsafe"
)

// constants for Logger
var (
	// Trace logs general information messages.
	Trace *log.Logger
	// Error logs error messages.
	Error *log.Logger
)

// nativeEndian used to detect native byte order
var nativeEndian binary.ByteOrder

// bufferSize used to set size of IO buffer - adjust to accommodate message payloads
var bufferSize = 8192

// IncomingMessage represents a message sent to the native host.
type IncomingMessage struct {
	Query string `json:"query"`
}

// OutgoingMessage respresents a response to an incoming message query.
type OutgoingMessage struct {
	Query    string `json:"query"`
	Response string `json:"response"`
}

// Init initializes logger and determines native byte order.
func Init(traceHandle io.Writer, errorHandle io.Writer) {
	Trace = log.New(traceHandle, "TRACE: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(errorHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	// determine native byte order so that we can read message size correctly
	var one int16 = 1
	b := (*byte)(unsafe.Pointer(&one))
	if *b == 0 {
		nativeEndian = binary.BigEndian
	} else {
		nativeEndian = binary.LittleEndian
	}
}

func main() {
	log.Println("Starting native messaging host")
	file, err := os.OpenFile("xcloudListener.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	log.Println(err)
	if err != nil {
		log.Println(err)
		Init(os.Stdout, os.Stderr)
		Error.Printf("Unable to create and/or open log file. Will log to Stdout and Stderr. Error: %v", err)
	} else {
		log.Println(err)
		Init(file, file)
		// ensure we close the log file when we're done
		defer file.Close()
	}

	Trace.Printf("Chrome native messaging host started. Native byte order: %v.", nativeEndian)
	handleRequests()
	Trace.Print("Chrome native messaging host exited.")
}


func homeEndpoint(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Home Endpoint hit")
	Trace.Print("Endpoint hit: homeEndpoint")
}

func actionEndpoint(w http.ResponseWriter, r *http.Request) {
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		Trace.Printf("Error reading action body: %s", err.Error())
		return
	}

	performAction(string(reqBody[:]))
}

func performAction(action string) {
	if action == "" {
		Trace.Printf("Action string is empty")
		return
	}
	
	Trace.Printf("Performing action: %s", action)
	msg := OutgoingMessage{ Query: action, Response: action}
	send(msg)
	Trace.Printf("Message query: %s", msg.Query)
	Trace.Printf("Message response: %s", msg.Response)
}

func handleRequests() {
	http.HandleFunc("/", homeEndpoint)
	http.HandleFunc("/action", actionEndpoint)
	
	log.Fatal(http.ListenAndServe(":9000", nil))
}

// read Creates a new buffered I/O reader and reads messages from Stdin.
func read() {
	v := bufio.NewReader(os.Stdin)
	// adjust buffer size to accommodate your json payload size limits; default is 4096
	s := bufio.NewReaderSize(v, bufferSize)
	Trace.Printf("IO buffer reader created with buffer size of %v.", s.Size())

	lengthBytes := make([]byte, 4)
	lengthNum := int(0)

	// we're going to indefinitely read the first 4 bytes in buffer, which gives us the message length.
	// if stdIn is closed we'll exit the loop and shut down host
	for b, err := s.Read(lengthBytes); b > 0 && err == nil; b, err = s.Read(lengthBytes) {
		// convert message length bytes to integer value
		lengthNum = readMessageLength(lengthBytes)
		Trace.Printf("Message size in bytes: %v", lengthNum)

		// If message length exceeds size of buffer, the message will be truncated.
		// This will likely cause an error when we attempt to unmarshal message to JSON.
		if lengthNum > bufferSize {
			Error.Printf("Message size of %d exceeds buffer size of %d. Message will be truncated and is unlikely to unmarshal to JSON.", lengthNum, bufferSize)
		}

		// read the content of the message from buffer
		content := make([]byte, lengthNum)
		_, err := s.Read(content)
		if err != nil && err != io.EOF {
			Error.Fatal(err)
		}

		// message has been read, now parse and process
		parseMessage(content)
	}

	Trace.Print("Stdin closed.")
}

// readMessageLength reads and returns the message length value in native byte order.
func readMessageLength(msg []byte) int {
	var length uint32
	buf := bytes.NewBuffer(msg)
	err := binary.Read(buf, nativeEndian, &length)
	if err != nil {
		Error.Printf("Unable to read bytes representing message length: %v", err)
	}
	return int(length)
}

// parseMessage parses incoming message
func parseMessage(msg []byte) {
	iMsg := decodeMessage(msg)
	Trace.Printf("Message received: %s", msg)

	// start building outgoing json message
	oMsg := OutgoingMessage{
		Query: iMsg.Query,
	}

	switch iMsg.Query {
	case "ping":
		oMsg.Response = "pong"
	case "hello":
		oMsg.Response = "goodbye"
	default:
		oMsg.Response = "42"
	}

	send(oMsg)
}

// decodeMessage unmarshals incoming json request and returns query value.
func decodeMessage(msg []byte) IncomingMessage {
	var iMsg IncomingMessage
	err := json.Unmarshal(msg, &iMsg)
	if err != nil {
		Error.Printf("Unable to unmarshal json to struct: %v", err)
	}
	return iMsg
}

// send sends an OutgoingMessage to os.Stdout.
func send(msg OutgoingMessage) {
	byteMsg := msgTextToBytes(msg.Response)
	writeMessageLength(byteMsg)

	var msgBuf bytes.Buffer
	_, err := msgBuf.Write(byteMsg)
	if err != nil {
		Error.Printf("Unable to write message length to message buffer: %v", err)
	}

	_, err = msgBuf.WriteTo(os.Stdout)
	if err != nil {
		Error.Printf("Unable to write message buffer to Stdout: %v", err)
	}
}

// dataToBytes marshals OutgoingMessage struct to slice of bytes
func msgTextToBytes(msgText string) []byte {
	byteMsg, err := json.Marshal(msgText)
	if err != nil {
		Error.Printf("Unable to marshal OutgoingMessage struct to slice of bytes: %v", err)
	}
	return byteMsg
}

// dataToBytes marshals OutgoingMessage struct to slice of bytes
func dataToBytes(msg OutgoingMessage) []byte {
	byteMsg, err := json.Marshal(msg)
	if err != nil {
		Error.Printf("Unable to marshal OutgoingMessage struct to slice of bytes: %v", err)
	}
	return byteMsg
}

// writeMessageLength determines length of message and writes it to os.Stdout.
func writeMessageLength(msg []byte) {
	err := binary.Write(os.Stdout, nativeEndian, uint32(len(msg)))
	if err != nil {
		Error.Printf("Unable to write message length to Stdout: %v", err)
	}
}
