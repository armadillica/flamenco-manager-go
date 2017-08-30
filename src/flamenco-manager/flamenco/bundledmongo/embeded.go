package bundledmongo

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	log "github.com/sirupsen/logrus"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// Runner provides an interface to start & stop the mongod executable.
type Runner struct {
	databasePath string
	localPort    int
	context      context.Context
	cancel       context.CancelFunc
	cmd          *exec.Cmd
	mutex        sync.Mutex
}

// CreateMongoRunner creates a new MongoRunner but doesn't start it yet.
func CreateMongoRunner(databasePath string, localPort int) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		databasePath: databasePath,
		localPort:    localPort,
		context:      ctx,
		cancel:       cancel,
	}
}

// Go starts mongodb and keeps it running in the background.
func (runner *Runner) Go() error {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	var err error

	ensureDirExists(runner.databasePath, "database path")
	ensureDirExists("mongodb-logs", "MongoDB logs path")

	localPortStr := fmt.Sprintf("%d", runner.localPort)
	log.Debugf("Local port string is %v", localPortStr)

	execPath, err := absMongoDbPath()
	if err != nil {
		return fmt.Errorf("Unable to determine path of MongoDB executable: %v", err)
	}
	log.Infof("MongoDB executable: %s", execPath)
	log.Infof("MongoDB database path: %s", runner.databasePath)
	log.Infof("MongoDB will be listening on port %d", runner.localPort)

	runner.cmd = exec.CommandContext(
		runner.context,
		execPath,
		"--port", localPortStr,
		"--bind_ip", "127.0.0.1",
		"--dbpath", runner.databasePath,
		"--quiet",
		"--logpath", "mongodb-logs/mongodb.log",
	)

	var stdout io.ReadCloser
	stdout, err = runner.cmd.StdoutPipe()
	if err != nil {
		log.Panicf("Unable to get pipe to MongoDB stdout: %s", err)
	}

	if err = runner.cmd.Start(); err != nil {
		log.Fatalf("Unable to start MongoDB: %s", err)
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		done := runner.context.Done()
		for {
			select {
			case <-done:
				log.Errorf("Runner context done, stopping reading!")
				return
			default:
			}

			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					log.Errorf("MongoDB: %s", err)
				}
				return
			}
			log.Warningf("MongoDB: %s", scanner.Text())
		}
	}()

	log.Infof("MongoDB is running at PID %d", runner.cmd.Process.Pid)
	return nil
}

func sendShutdownCommand(session *mgo.Session) {
	// session.Anything() can cause a panic if the session was closed.
	// This is fine in this case, since a close session means a shut down server.
	defer func() {
		e := recover()
		if e != nil {
			log.Errorf("Panic in sendShutdownCommand: %v", e)
			return
		}
	}()

	mySession := session.Clone()
	defer mySession.Close()

	log.Info("Sending shutdown command to MongoDB")
	if err := mySession.Ping(); err != nil {
		log.Errorf("No ping: %v", err)
		return
	}

	// Session is alive, we can use it to tell the server to shut down.
	var result bson.M
	if err := mySession.DB("admin").Run("shutdown", &result); err != nil && err != io.EOF {
		log.Infof("Unable to send MongoDB a shutdown command: %v", err)
	}
}

// Close gracefully stops mongod.
func (runner *Runner) Close(session *mgo.Session) {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()

	if runner.cmd == nil {
		log.Info("Stopping MongoDB even before it started.")
		return
	}

	// Not really checking for errors, just shut it all down.
	go sendShutdownCommand(session)
	if err := runner.cmd.Wait(); err != nil {
		log.Errorf("Error waiting for mongod: %v", err)
		return
	}

	log.Infof("MongoDB shut down gracefully")
}
