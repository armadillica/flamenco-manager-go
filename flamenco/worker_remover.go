/* (c) 2019, Blender Foundation - Sybren A. Stüvel
 *
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 *
 * The above copyright notice and this permission notice shall be
 * included in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
 * CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
 * TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package flamenco

import (
	"time"

	log "github.com/sirupsen/logrus"
	mgo "gopkg.in/mgo.v2"
)

const (
	// Initial delay to allow workers to come back online after the Manager was down.
	workerRemoverInitialSleep  = 5 * time.Minute
	workerRemoverCheckInterval = 30 * time.Second

	workerCleanupTaskRequeueReason = "worker is being auto-removed"
)

// WorkerRemover periodically removes offline workers.
type WorkerRemover struct {
	closable
	config    *Conf
	session   *mgo.Session
	scheduler *TaskScheduler
	logFields log.Fields
}

// CreateWorkerRemover creates a WorkerRemover, or returns nil if the configuration disables automatic worker removal.
func CreateWorkerRemover(config *Conf, session *mgo.Session, scheduler *TaskScheduler) *WorkerRemover {
	logFields := log.Fields{
		"worker_cleanup_max_age": config.WorkerCleanupMaxAge,
		"worker_cleanup_status":  config.WorkerCleanupStatus,
	}
	logger := log.WithFields(logFields)
	if config.WorkerCleanupMaxAge == 0*time.Second || len(config.WorkerCleanupStatus) == 0 {
		logger.Info("workers will not be auto-removed")
		return nil
	}
	logger.Info("workers will be auto-removed")
	return &WorkerRemover{
		makeClosable(),
		config,
		session,
		scheduler,
		logFields,
	}
}

// Close signals the WorkerRemover goroutine to stop and waits for it to close.
func (wr *WorkerRemover) Close() {
	log.Debug("WorkerRemover: Close() called.")
	wr.closableCloseAndWait()
	log.Debug("WorkerRemover: shutdown complete.")
}

// Go starts a goroutine that periodically checks workers.
func (wr *WorkerRemover) Go() {
	wr.closableAdd(1)
	go func() {
		session := wr.session.Copy()
		db := session.DB("")
		defer session.Close()
		defer wr.closableDone()
		defer log.Info("WorkerRemover: shutting down.")

		// Start with a delay, so that workers get a chance to push their updates
		// after the manager has started up.
		timer := Timer("WorkerRemover", workerRemoverCheckInterval, workerRemoverInitialSleep, &wr.closable)

		for range timer {
			wr.cleanupWorkers(db)
		}
	}()
}

func (wr *WorkerRemover) cleanupWorkers(db *mgo.Database) {
	// Any worker last seen before the threshold will be deleted.
	threshold := time.Now().Add(-wr.config.WorkerCleanupMaxAge)
	logger := log.WithFields(wr.logFields).WithFields(log.Fields{
		"last_activity_threshold": threshold,
	})
	logger.Debug("WorkerRemover: cleaning up workers")

	worker := Worker{}
	workersColl := db.C("flamenco_workers")
	query := workersColl.Find(M{
		"status":        M{"$in": wr.config.WorkerCleanupStatus},
		"last_activity": M{"$lt": threshold},
	})
	iter := query.Iter()
	for iter.Next(&worker) {
		workerLogger := logger.WithFields(log.Fields{
			"worker_id":            worker.ID.Hex(),
			"worker_status":        worker.Status,
			"worker_last_activity": worker.LastActivity,
		})
		workerLogger.Warning("WorkerRemover: removing worker")
		worker.returnAllTasks(wr.logFields, db, wr.scheduler, workerCleanupTaskRequeueReason)
		if err := workersColl.RemoveId(worker.ID); err != nil {
			workerLogger.WithError(err).Error("unable to auto-remove worker")
		}
	}
	err := iter.Close()
	if err != nil {
		logger.WithError(err).Warning("WorkerRemover: unable to query for to-be-cleaned-up workers")
	}
}
