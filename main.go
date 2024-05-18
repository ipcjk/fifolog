package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"syscall"
	"time"
)

var fileDest io.WriteCloser
var udpConn net.Conn
var fifo, udpDest, outFile *string

func main() {
	fifo = flag.String("i", "access.log", "Input fifo")
	outFile = flag.String("o", "log", "Path to the file that will be extended with a year-month-day format")
	udpDest = flag.String("u", "", "udp destination, e.g. 127.0.0.1:3309")
	flag.Parse()

	lineChannel := make(chan string, 1)
	writeChannel := make(chan string, 1)
	rotateChannel := make(chan struct{})

	/* open destinations */
	mw, err := setDestinations()
	if err != nil {
		log.Fatal(err)
	}

	/* sleep and check for a change of the current day */
	go checkTime(rotateChannel)
	// go checkSeconds(rotateChannel)
	
	/* prepare line  */
	go consumeLogLine(lineChannel, writeChannel)

	/* write line */
	go writeLine(mw, writeChannel, rotateChannel)

	/* wait and read from the input  */
	for {

		file, err := createFifo()
		if err != nil {
			log.Fatal(err)
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lineChannel <- scanner.Text()
		}

		/* if we are here, file input has received some kind of close */
		file.Close()
		scanner = nil
	}

}

func createFifo() (io.ReadCloser, error) {
	fi, err := os.Stat(*fifo)

	if os.IsNotExist(err) {
		/* try to recover the error and create pipe */
		err = syscall.Mkfifo(*fifo, 0666)

		if err != nil {
			return nil, fmt.Errorf("cant create fifo file: %s", err)
		}

		fmt.Println("Created pipe ", *fifo)

	} else if err != nil {
		return nil, fmt.Errorf("unknown error when opening named pipe: %s", err)
	}

	if fi != nil && fi.Mode()&os.ModeNamedPipe == 0 {
		return nil, fmt.Errorf("the input file is not a named pipe: %s", err)
	}

	file, err := os.Open(*fifo)
	if err != nil {
		return nil, fmt.Errorf("cant open named pipe: %s", err)
	}

	return file, nil
}

func checkTime(rotateLog chan struct{}) {
	_, _, day := time.Now().Date()
	for {
		_, _, newDay := time.Now().Date()
		if day != newDay {
			day = newDay
			rotateLog <- struct{}{}
		}
		time.Sleep(time.Duration(time.Second * 5))
	}
}

func writeLine(mw io.Writer, wc chan string, rotateLog chan struct{}) {
	var err error

	for {
		select {
		case line := <-wc:
			_, err = fmt.Fprint(mw, line, "\n")
			if err != nil {
				fmt.Fprint(os.Stderr, "Writing error, closing sockets/file and reopen")
				mw, err = setDestinations()
				if err != nil {
					return
				}
			}
		case <-rotateLog:
			mw, err = setDestinations()
			if err != nil {
				return
			}
		}
	}

}

func setDestinations() (io.Writer, error) {
	var err error
	year, month, day := time.Now().Date()

	if *fifo != "" {
		if fileDest != nil {
			err := fileDest.Close()
			if err != nil {
				fmt.Println("Error closing file")
			}
		}

		fileDest, err = openDestinationLogFile(year, month, day)
		if err != nil {
			return nil, err
		}
	}

	if *udpDest != "" {
		if udpConn != nil {
			udpConn.Close()
		}
		udpConn, err = net.Dial("udp", *udpDest)
		if err != nil {
			return nil, err
		}
		if fileDest != nil {
			return io.MultiWriter(fileDest, udpConn), nil
		}
		return io.MultiWriter(udpConn), nil
	}

	/* could also log to a tcp socket, syslog server, etc */
	if fileDest != nil {
		return io.MultiWriter(fileDest), nil
	}
	return nil, fmt.Errorf("Error: %s", "No output given")
}

func consumeLogLine(lc chan string, wc chan string) {
	for v := range lc {
		wc <- v
	}
}

func openDestinationLogFile(year int, month time.Month, day int) (wc io.WriteCloser, err error) {
	return os.OpenFile(fmt.Sprintf("%s-%d-%02d-%02d", *outFile, year, month, day),
		os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
}
