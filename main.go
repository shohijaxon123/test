package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

type PageData struct {
	Handler string
	Query   string
	Headers string
	Body    string
}

func main() {
	host := "0.0.0.0"
	port := "9999"
	if err := execute(host, port); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func execute(host, port string) (err error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		log.Println(err)
		return err
	}

	defer func(ls net.Listener) {
		err := ls.Close()
		if err != nil {
			log.Println(err)
		}
	}(listener)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		err = handle(conn)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}
func handle(conn net.Conn) (err error) {
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			if err == nil {
				err = cerr
			}
			log.Println(err)
		}
	}()

	buff := make([]byte, 4096)
	n, err := conn.Read(buff)
	if errors.Is(err, io.EOF) {
		log.Println("Connection closed by client")
		return nil
	}
	if err != nil {
		log.Println(err)
		return err
	}

	data := buff[:n]

	requestDelimiter := []byte{'\r', '\n'}
	requestLineEnd := bytes.Index(data, requestDelimiter)
	if requestLineEnd == -1 {
		log.Println("Invalid request")
		return nil
	}

	// Читает Request-Line
	requestLine := string(data[:requestLineEnd])
	parts := strings.Split(requestLine, " ")
	if len(parts) < 3 {
		log.Println("Invalid request line")
		return nil
	}

	method := parts[0]
	fullPath := parts[1]

	if method != "GET" && method != "POST" {
		log.Println("Unsupported method")
		return nil
	}

	// Читает заголовок
	headers := make(map[string]string)
	headersSection := data[requestLineEnd+2:]
	headersEnd := bytes.Index(headersSection, []byte("\r\n\r\n"))
	if headersEnd == -1 {
		log.Println("Headers not found")
		return nil
	}

	rawHeaders := string(headersSection[:headersEnd])
	for _, line := range strings.Split(rawHeaders, "\r\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers[key] = value
		}
	}

	// Body
	body := ""
	bodyStart := headersEnd + 4
	if bodyStart < len(headersSection) {
		body = string(headersSection[bodyStart:])
	}

	handlerName := "unknown"
	query := ""

	pathParts := strings.SplitN(fullPath, "?", 2)
	pathOnly := pathParts[0]
	if len(pathParts) > 1 {
		query = strings.ReplaceAll(pathParts[1], "&", "\n")
		query = strings.ReplaceAll(query, "=", " = ")
	}

	pathSegments := strings.Split(pathOnly, "/")
	if len(pathSegments) >= 3 && pathSegments[1] == "handler" {
		handlerName = pathSegments[2]
	}

	file, err := os.ReadFile("static/layout_handler.html")
	if err != nil {
		log.Println("can't find layout_handler.html")
		return err
	}

	headersStr := ""
	for key, value := range headers {
		headersStr += fmt.Sprintf("%s: %s\n", key, value)
	}

	file = bytes.ReplaceAll(file, []byte("{{handler}}"), []byte(handlerName))
	file = bytes.ReplaceAll(file, []byte("{{query}}"), []byte(query))
	file = bytes.ReplaceAll(file, []byte("{{headers}}"), []byte(headersStr))
	file = bytes.ReplaceAll(file, []byte("{{body}}"), []byte(body))

	_, err = conn.Write([]byte("HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/html\r\n" +
		"Content-Length: " + strconv.Itoa(len(file)) + "\r\n" +
		"\r\n" +
		string(file)))

	if err != nil {
		log.Println("can't write file")
		return err
	}

	return nil
}
