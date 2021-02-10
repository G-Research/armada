package repository

import (
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"

	"github.com/G-Research/armada/internal/common/util"
	"github.com/G-Research/armada/pkg/api/lookout"
)

func TestGetJobSetInfos_GetNoJobSetsIfQueueDoesNotExist(t *testing.T) {
	withDatabase(t, func(db *goqu.Database) {
		jobStore := NewSQLJobStore(db)

		NewJobSimulator(t, jobStore).
			CreateJob("queue-1")

		NewJobSimulator(t, jobStore).
			CreateJob("queue-2").
			Pending(cluster, k8sId1)

		jobRepo := NewSQLJobRepository(db, &DefaultClock{})

		jobSetInfos, err := jobRepo.GetJobSetInfos(ctx, &lookout.GetJobSetsRequest{Queue: queue})
		assert.NoError(t, err)
		assert.Empty(t, jobSetInfos)
	})
}

func TestGetJobSetInfos_GetsJobSetWithNoFinishedJobs(t *testing.T) {
	withDatabase(t, func(db *goqu.Database) {
		jobStore := NewSQLJobStore(db)

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set")

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set").
			Pending(cluster, k8sId1)

		jobRepo := NewSQLJobRepository(db, &DefaultClock{})

		jobSetInfos, err := jobRepo.GetJobSetInfos(ctx, &lookout.GetJobSetsRequest{Queue: queue})
		assert.NoError(t, err)
		assertJobSetInfosAreEqual(t, &lookout.JobSetInfo{
			Queue:         queue,
			JobSet:        "job-set",
			JobsQueued:    1,
			JobsPending:   1,
			JobsRunning:   0,
			JobsSucceeded: 0,
			JobsFailed:    0,
		}, jobSetInfos[0])
	})
}

func TestGetJobSetInfos_GetsJobSetWithOnlyFinishedJobs(t *testing.T) {
	withDatabase(t, func(db *goqu.Database) {
		jobStore := NewSQLJobStore(db)

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set").
			Running(cluster, k8sId1, node).
			Succeeded(cluster, k8sId1, node)

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set").
			Pending(cluster, k8sId2).
			Failed(cluster, k8sId2, node, "some error")

		jobRepo := NewSQLJobRepository(db, &DefaultClock{})

		jobSetInfos, err := jobRepo.GetJobSetInfos(ctx, &lookout.GetJobSetsRequest{Queue: queue})
		assert.NoError(t, err)
		assertJobSetInfosAreEqual(t, &lookout.JobSetInfo{
			Queue:         queue,
			JobSet:        "job-set",
			JobsQueued:    0,
			JobsPending:   0,
			JobsRunning:   0,
			JobsSucceeded: 1,
			JobsFailed:    1,
		}, jobSetInfos[0])
	})
}

func TestGetJobSetInfos_JobSetsCounts(t *testing.T) {
	withDatabase(t, func(db *goqu.Database) {
		jobStore := NewSQLJobStore(db)
		jobRepo := NewSQLJobRepository(db, &DefaultClock{})

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1").
			Pending(cluster, "a1").
			UnableToSchedule(cluster, "a1", node).
			Pending(cluster, "a2")

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1").
			Pending(cluster, "b1").
			UnableToSchedule(cluster, "b1", node).
			Running(cluster, "b2", node)

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1")

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1").
			Pending(cluster, "c1").
			UnableToSchedule(cluster, "c1", node).
			Running(cluster, "c2", node).
			Succeeded(cluster, "c2", node)

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1").
			Pending(cluster, "d1").
			UnableToSchedule(cluster, "d1", node).
			Running(cluster, "d2", node).
			Failed(cluster, "d2", node, "something bad")

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1").
			Pending(cluster, "e1").
			UnableToSchedule(cluster, "e1", node).
			Running(cluster, "e2", node).
			Cancelled()

		jobSetInfos, err := jobRepo.GetJobSetInfos(ctx, &lookout.GetJobSetsRequest{Queue: queue})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(jobSetInfos))
		assertJobSetInfosAreEqual(t, &lookout.JobSetInfo{
			Queue:         queue,
			JobSet:        "job-set-1",
			JobsQueued:    1,
			JobsPending:   1,
			JobsRunning:   1,
			JobsSucceeded: 1,
			JobsFailed:    1,
		}, jobSetInfos[0])
	})
}

func TestGetJobSetInfos_MultipleJobSetsCounts(t *testing.T) {
	withDatabase(t, func(db *goqu.Database) {
		jobStore := NewSQLJobStore(db)
		jobRepo := NewSQLJobRepository(db, &DefaultClock{})

		// Job set 1
		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1")

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1")

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1").
			Pending(cluster, "a1").
			UnableToSchedule(cluster, "a1", node).
			Pending(cluster, "a2")

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1").
			Pending(cluster, "b1").
			UnableToSchedule(cluster, "b1", node).
			Running(cluster, "b2", node)

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1").
			Pending(cluster, "c1").
			UnableToSchedule(cluster, "c1", node).
			Running(cluster, "c2", node).
			Succeeded(cluster, "c2", node)

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-1").
			Pending(cluster, "d1").
			UnableToSchedule(cluster, "d1", node).
			Running(cluster, "d2", node).
			Failed(cluster, "d2", node, "something bad")

		// Job set 2
		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-2")

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-2").
			Pending(cluster, "e1")

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-2").
			Pending(cluster, "f1").
			UnableToSchedule(cluster, "f1", node).
			Running(cluster, "f2", node).
			Succeeded(cluster, "f2", node)

		NewJobSimulator(t, jobStore).
			CreateJobWithJobSet(queue, "job-set-2").
			Pending(cluster, "h1").
			UnableToSchedule(cluster, "h1", node).
			Running(cluster, "h2", node).
			Failed(cluster, "h2", node, "something bad")

		jobSetInfos, err := jobRepo.GetJobSetInfos(ctx, &lookout.GetJobSetsRequest{Queue: queue})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(jobSetInfos))

		assertJobSetInfosAreEqual(t, &lookout.JobSetInfo{
			Queue:         queue,
			JobSet:        "job-set-1",
			JobsQueued:    2,
			JobsPending:   1,
			JobsRunning:   1,
			JobsSucceeded: 1,
			JobsFailed:    1,
		}, jobSetInfos[0])

		assertJobSetInfosAreEqual(t, &lookout.JobSetInfo{
			Queue:         queue,
			JobSet:        "job-set-2",
			JobsQueued:    1,
			JobsPending:   1,
			JobsRunning:   0,
			JobsSucceeded: 1,
			JobsFailed:    1,
		}, jobSetInfos[1])
	})
}

func TestGetJobSetInfos_StatsWithNoRunningOrQueuedJobs(t *testing.T) {
	withDatabase(t, func(db *goqu.Database) {
		jobStore := NewSQLJobStore(db)
		jobRepo := NewSQLJobRepository(db, &DefaultClock{})

		NewJobSimulator(t, jobStore).
			CreateJob(queue).
			UnableToSchedule(cluster, k8sId1, node).
			Pending(cluster, k8sId2).
			Running(cluster, k8sId2, node).
			Succeeded(cluster, k8sId2, node)

		jobSets, err := jobRepo.GetJobSetInfos(ctx, &lookout.GetJobSetsRequest{Queue: queue})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(jobSets))

		assert.Nil(t, jobSets[0].RunningStats)
		assert.Nil(t, jobSets[0].QueuedStats)
	})
}

func TestGetJobSetInfos_GetRunningStats(t *testing.T) {
	withDatabase(t, func(db *goqu.Database) {
		jobStore := NewSQLJobStore(db)

		currentTime := someTime.Add(20 * time.Minute)

		jobRepo := NewSQLJobRepository(db, &DummyClock{currentTime})

		for i := 0; i < 11; i++ {
			k8sId := util.NewULID()
			otherK8sId := util.NewULID()
			itTime := someTime.Add(time.Duration(i) * time.Minute)
			NewJobSimulator(t, jobStore).
				CreateJobAtTime(queue, itTime).
				UnableToScheduleAtTime(cluster, k8sId, node, itTime).
				PendingAtTime(cluster, otherK8sId, itTime).
				RunningAtTime(cluster, otherK8sId, node, itTime)

			NewJobSimulator(t, jobStore).
				CreateJobAtTime(queue, someTime)

			NewJobSimulator(t, jobStore).
				CreateJobAtTime(queue, someTime).
				CancelledAtTime(someTime)
		}

		// All the same, except for last
		for i := 0; i < 10; i++ {
			k8sId := util.NewULID()
			otherK8sId := util.NewULID()
			NewJobSimulator(t, jobStore).
				CreateJobWithOpts(queue, util.NewULID(), "job-set-2", someTime).
				UnableToScheduleAtTime(cluster, k8sId, node, someTime).
				PendingAtTime(cluster, otherK8sId, someTime).
				RunningAtTime(cluster, otherK8sId, node, someTime)

			NewJobSimulator(t, jobStore).
				CreateJobWithOpts(queue, util.NewULID(), "job-set-2", someTime)

			NewJobSimulator(t, jobStore).
				CreateJobWithOpts(queue, util.NewULID(), "job-set-2", someTime).
				CancelledAtTime(someTime)
		}

		otherTime := someTime.Add(10 * time.Minute)
		NewJobSimulator(t, jobStore).
			CreateJobWithOpts(queue, util.NewULID(), "job-set-2", otherTime).
			UnableToScheduleAtTime(cluster, k8sId1, node, otherTime).
			PendingAtTime(cluster, k8sId2, otherTime).
			RunningAtTime(cluster, k8sId2, node, otherTime)

		jobSets, err := jobRepo.GetJobSetInfos(ctx, &lookout.GetJobSetsRequest{Queue: queue})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(jobSets))

		assertDurationStatsAreEqual(t, &lookout.DurationStats{
			Shortest: types.DurationProto(10 * time.Minute),
			Longest:  types.DurationProto(20 * time.Minute),
			Average:  types.DurationProto(15 * time.Minute),
			Median:   types.DurationProto(15 * time.Minute),
			Q1:       types.DurationProto(12*time.Minute + 30*time.Second),
			Q3:       types.DurationProto(17*time.Minute + 30*time.Second),
		}, jobSets[0].RunningStats)

		assertDurationStatsAreEqual(t, &lookout.DurationStats{
			Shortest: types.DurationProto(10 * time.Minute),
			Longest:  types.DurationProto(20 * time.Minute),
			Average:  types.DurationProto(19*time.Minute + 5*time.Second),
			Median:   types.DurationProto(20 * time.Minute),
			Q1:       types.DurationProto(20 * time.Minute),
			Q3:       types.DurationProto(20 * time.Minute),
		}, jobSets[1].RunningStats)
	})
}

func TestGetJobSetInfos_GetQueuedStats(t *testing.T) {
	withDatabase(t, func(db *goqu.Database) {
		jobStore := NewSQLJobStore(db)

		someTime := time.Now()
		currentTime := someTime.Add(30 * time.Minute)

		jobRepo := NewSQLJobRepository(db, &DummyClock{currentTime})

		for i := 0; i < 11; i++ {
			k8sId := util.NewULID()
			otherK8sId := util.NewULID()
			itTime := someTime.Add(time.Duration(i) * time.Minute)
			NewJobSimulator(t, jobStore).
				CreateJobAtTime(queue, someTime).
				UnableToScheduleAtTime(cluster, k8sId, node, someTime).
				PendingAtTime(cluster, otherK8sId, someTime).
				RunningAtTime(cluster, otherK8sId, node, someTime)

			NewJobSimulator(t, jobStore).
				CreateJobAtTime(queue, itTime)

			NewJobSimulator(t, jobStore).
				CreateJobAtTime(queue, someTime).
				CancelledAtTime(someTime)
		}

		for i := 0; i < 10; i++ {
			k8sId := util.NewULID()
			otherK8sId := util.NewULID()
			NewJobSimulator(t, jobStore).
				CreateJobWithOpts(queue, util.NewULID(), "job-set-2", someTime).
				UnableToSchedule(cluster, k8sId, node).
				Pending(cluster, otherK8sId).
				Running(cluster, otherK8sId, node)

			NewJobSimulator(t, jobStore).
				CreateJobWithOpts(queue, util.NewULID(), "job-set-2", someTime)

			NewJobSimulator(t, jobStore).
				CreateJobWithOpts(queue, util.NewULID(), "job-set-2", someTime).
				Cancelled()
		}

		NewJobSimulator(t, jobStore).
			CreateJobWithOpts(queue, util.NewULID(), "job-set-2", someTime.Add(20*time.Minute))

		jobSets, err := jobRepo.GetJobSetInfos(ctx, &lookout.GetJobSetsRequest{Queue: queue})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(jobSets))

		assertDurationStatsAreEqual(t, &lookout.DurationStats{
			Shortest: types.DurationProto(20 * time.Minute),
			Longest:  types.DurationProto(30 * time.Minute),
			Average:  types.DurationProto(25 * time.Minute),
			Median:   types.DurationProto(25 * time.Minute),
			Q1:       types.DurationProto(22*time.Minute + 30*time.Second),
			Q3:       types.DurationProto(27*time.Minute + 30*time.Second),
		}, jobSets[0].QueuedStats)

		assertDurationStatsAreEqual(t, &lookout.DurationStats{
			Shortest: types.DurationProto(10 * time.Minute),
			Longest:  types.DurationProto(30 * time.Minute),
			Average:  types.DurationProto(28*time.Minute + 11*time.Second),
			Median:   types.DurationProto(30 * time.Minute),
			Q1:       types.DurationProto(30 * time.Minute),
			Q3:       types.DurationProto(30 * time.Minute),
		}, jobSets[1].QueuedStats)
	})
}

func assertDurationStatsAreEqual(t *testing.T, expected *lookout.DurationStats, actual *lookout.DurationStats) {
	t.Helper()
	AssertProtoDurationsApproxEqual(t, expected.Longest, actual.Longest)
	AssertProtoDurationsApproxEqual(t, expected.Shortest, actual.Shortest)
	AssertProtoDurationsApproxEqual(t, expected.Average, actual.Average)
	AssertProtoDurationsApproxEqual(t, expected.Median, actual.Median)
	AssertProtoDurationsApproxEqual(t, expected.Q1, actual.Q1)
	AssertProtoDurationsApproxEqual(t, expected.Q3, actual.Q3)
}

func assertJobSetInfosAreEqual(t *testing.T, expected *lookout.JobSetInfo, actual *lookout.JobSetInfo) {
	t.Helper()
	assert.Equal(t, expected.JobSet, actual.JobSet)
	assert.Equal(t, expected.Queue, actual.Queue)
	assert.Equal(t, expected.JobsQueued, actual.JobsQueued)
	assert.Equal(t, expected.JobsPending, actual.JobsPending)
	assert.Equal(t, expected.JobsRunning, actual.JobsRunning)
	assert.Equal(t, expected.JobsSucceeded, actual.JobsSucceeded)
	assert.Equal(t, expected.JobsFailed, actual.JobsFailed)
}
