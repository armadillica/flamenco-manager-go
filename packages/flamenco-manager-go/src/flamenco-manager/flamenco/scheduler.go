package flamenco

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	auth "github.com/abbot/go-http-auth"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

/* Timestamp of the last time we kicked the task downloader because there weren't any
 * tasks left for workers. */
var last_upstream_check time.Time

type TaskScheduler struct {
	config   *Conf
	upstream *UpstreamConnection
	session  *mgo.Session
}

func CreateTaskScheduler(config *Conf, upstream *UpstreamConnection, session *mgo.Session) *TaskScheduler {
	return &TaskScheduler{
		config,
		upstream,
		session,
	}
}

func (ts *TaskScheduler) ScheduleTask(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	log.Printf("%s Worker %s asking for a task", r.RemoteAddr, r.Username)

	mongo_sess := ts.session.Copy()
	defer mongo_sess.Close()
	db := mongo_sess.DB("")

	// Fetch the worker's info
	projection := bson.M{"platform": 1, "supported_job_types": 1, "address": 1}
	worker, err := FindWorker(r.Username, projection, db)
	if err != nil {
		log.Printf("%s ScheduleTask: Unable to find worker: %s\n", r.RemoteAddr, err)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Unable to find worker: %s", err)
		return
	}
	WorkerSeen(worker, r.RemoteAddr, db)

	var task *Task
	var was_changed bool
	for attempt := 0; attempt < 1000; attempt++ {
		// Fetch the first available task of a supported job type.
		task = ts.fetchTaskFromQueueOrManager(w, r, db, worker)
		if task == nil {
			// A response has already been written to 'w'.
			return
		}

		was_changed = ts.upstream.RefetchTask(task)
		if !was_changed {
			break
		}

		log.Printf("Task %s was changed, reexamining queue.", task.Id.Hex())
	}
	if was_changed {
		log.Printf("Infinite loop detected, tried 1000 tasks and they all changed...")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Perform variable replacement on the task.
	ReplaceVariables(ts.config, task, worker)

	// update the worker_id field of the task.
	tasks_coll := db.C("flamenco_tasks")
	if err := tasks_coll.UpdateId(task.Id, bson.M{"$set": bson.M{"worker_id": worker.Id}}); err != nil {
		log.Printf("Unable to set worker_id=%s on task %s: %s", worker.Id, task.Id, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Set it to this worker.
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.Encode(task)

	log.Printf("%s assigned task %s to worker %s", r.RemoteAddr, task.Id.Hex(), r.Username)
}

/**
 * Fetches a task from either the queue, or if it is empty, from the manager.
 */
func (ts *TaskScheduler) fetchTaskFromQueueOrManager(
	w http.ResponseWriter, r *auth.AuthenticatedRequest,
	db *mgo.Database, worker *Worker) *Task {

	if len(worker.SupportedJobTypes) == 0 {
		log.Printf("%s Warning: worker %s has no supported job types.",
			r.RemoteAddr, worker.Id.Hex())
		return nil
	}

	task := &Task{}
	tasks_coll := db.C("flamenco_tasks")

	// TODO Sybren: also include active tasks that are assigned to this worker.
	query := bson.M{
		"status":   bson.M{"$in": []string{"queued", "claimed-by-manager"}},
		"job_type": bson.M{"$in": worker.SupportedJobTypes},
	}
	change := mgo.Change{
		Update:    bson.M{"$set": bson.M{"status": "active"}},
		ReturnNew: true,
	}

	dtrt := ts.config.DownloadTaskRecheckThrottle

	for attempt := 0; attempt < 2; attempt++ {
		// TODO: possibly sort on something else.
		info, err := tasks_coll.Find(query).Sort("-priority").Limit(1).Apply(change, &task)
		if err == mgo.ErrNotFound {
			if attempt == 0 && dtrt >= 0 && time.Now().Sub(last_upstream_check) > dtrt {
				// On first attempt: try fetching new tasks from upstream, then re-query the DB.
				log.Printf("%s No more tasks available for %s, checking upstream\n",
					r.RemoteAddr, r.Username)
				last_upstream_check = time.Now()
				ts.upstream.KickDownloader(true)
				continue
			}

			log.Printf("%s Really no more tasks available for %s\n", r.RemoteAddr, r.Username)
			w.WriteHeader(204)
			return nil
		} else if err != nil {
			log.Printf("%s Error fetching task for %s: %s // %s\n", r.RemoteAddr, r.Username, err, info)
			w.WriteHeader(500)
			return nil
		}

		break
	}

	return task
}
