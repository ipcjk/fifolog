package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
	"syscall"
)

var fileDest io.WriteCloser
var udpConn net.Conn
var fifo, udpDest, outFile *string

func main() {
	fifo = flag.String("i", "access.log", "Input fifo")
	outFile = flag.String("o", "log", "Output filename that will be extended with year-month-day")
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
		err = syscall.Mkfifo(*fifo, 0600)

		if err != nil {
			return nil, fmt.Errorf("Cant create fifo file: %s", err)
		}

		fmt.Println("Created pipe ", *fifo)

	} else if err != nil {
		return nil, fmt.Errorf("Unknown error when opening named pipe: %s", err)
	}

	if fi != nil && fi.Mode() & os.ModeNamedPipe == 0 {
		return nil, fmt.Errorf("The input file is not a named pipe: %s", err)
	}

	file, err := os.Open(*fifo)
	if err != nil {
		return nil, fmt.Errorf("Cant open named pipe: %s", err)
	}

	return file, nil
}

func checkSeconds(rotateLog chan struct{}) {
	for {
		timeNow := time.Now()
		if timeNow.Second()%5 == 0 {
			rotateLog <- struct{}{}
		}
		time.Sleep(time.Duration(time.Second))
	}
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

func writeLine(mw io.Writer, wc chan string, rotateLog chan struct{})  error {
	var err error

	for {
		select {
		case line := <-wc:
			_, err = fmt.Fprint(mw, line, "\n")
			if err != nil {
				fmt.Fprint(os.Stderr, "Writing error, closing sockets/file and reopen")
				mw, err = setDestinations()
				if err != nil {
					return err
				}
			}
		case <-rotateLog:
			mw, err = setDestinations()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func setDestinations() (io.Writer, error) {
	var err error = nil
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
		} else {
			return io.MultiWriter(udpConn), nil
		}
	}

	/* could also log to a tcp socket, syslog server, etc */
	if fileDest != nil {
		return io.MultiWriter(fileDest), nil
	} else {
		return nil, fmt.Errorf("Error: %s", "No output given")
	}
}

func consumeLogLine(lc chan string, wc chan string) {
	for v := range lc {
		wc <- v
	}
}

func openDestinationLogFile(year int, month time.Month, day int) (wc io.WriteCloser, err error) {
	return os.OpenFile(fmt.Sprintf("%s-%d-%02d-%02d", *outFile, year, month, day), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
}
