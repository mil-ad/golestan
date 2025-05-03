package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

const socketPath = "/tmp/gojeh.sock"

type App struct {
	toggleTimer    chan bool
	nextSession    chan bool
	tick           chan bool
	timerIsRunning bool
	sessions       []Session
	currSessionIdx int
}

func NewApp() *App {
	return &App{
		toggleTimer:    make(chan bool),
		nextSession:    make(chan bool),
		tick:           make(chan bool),
		timerIsRunning: false,
		sessions: []Session{
			{Icon: "🍅", InitialSeconds: 25 * 60},
			{Icon: "☕", InitialSeconds: 3 * 60},
		},
		currSessionIdx: 0,
	}
}

type Session struct {
	Icon           string
	InitialSeconds int
}

func (app *App) print(seconds int) {
	if seconds > 0 {
		fmt.Printf("%s %02d:%02d\n", app.sessions[app.currSessionIdx].Icon, seconds/60, seconds%60)
	} else {
		fmt.Printf("%s -%02d:%02d\n", app.sessions[app.currSessionIdx].Icon, -seconds/60, -seconds%60)
	}
}

func notify() {
	cmd := exec.Command("notify-send", "-t", "0", "-u", "critical", "Pomodoro", "Timer reached zero")
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func (app *App) NextSession() (seconds int) {
	app.currSessionIdx = (app.currSessionIdx + 1) % 2
	return app.sessions[app.currSessionIdx].InitialSeconds
}

func (app *App) run() {
	seconds := app.sessions[0].InitialSeconds
	app.print(seconds)

	tickInOneSec := func() {
		time.Sleep(1 * time.Second)
		app.tick <- true
	}

	for {
		select {
		case <-app.toggleTimer:
			if app.timerIsRunning {
				app.timerIsRunning = false
			} else {
				app.timerIsRunning = true

				// Decrementing the seconds before timer ticks so that the user
				// has a better UX by immediately seeing some feedback when the
				// timer is toggled.
				seconds--

				go tickInOneSec()
			}
			app.print(seconds)
		case <-app.nextSession:
			app.timerIsRunning = false
			seconds = app.NextSession()
			app.print(seconds)
		case <-app.tick:
			if app.timerIsRunning {
				seconds--
				if seconds == 0 {
					notify()
				}
				app.print(seconds)
				go tickInOneSec()
			}
		}
	}
}

func (app *App) handleExtCommand(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	message, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading from connection: %v", err)
		return
	}

	message = strings.TrimSpace(message)

	switch message {
	case "toggle":
		log.Println("Received toggle command")
		app.toggleTimer <- true
	case "next":
		log.Println("Received next command")
		app.nextSession <- true
	default:
		log.Printf("Received unknown command: %s", message)
		conn.Write([]byte("Unknown command. Please use 'start' or 'stop'.\n"))
	}
}

func main() {

	// // Parse flags
	// var durFlag = flag.String("d", "25m", "help message for flag n")
	// flag.Parse()
	// d, _ := time.ParseDuration(*durFlag)
	// fmt.Println(d)

	err := os.RemoveAll(socketPath)
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	app := NewApp()
	go app.run()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Failed to accept connection:", err)
			continue
		}
		go app.handleExtCommand(conn)
	}
}
