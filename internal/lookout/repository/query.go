package repository

import (
	"database/sql"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/lib/pq"

	"github.com/G-Research/armada/pkg/api"
	"github.com/G-Research/armada/pkg/api/lookout"
)

type JobRepository interface {
	GetQueueStats() ([]*lookout.QueueInfo, error)
	GetJobsInQueue(opts *lookout.GetJobsInQueueRequest) ([]*lookout.JobInfo, error)
}

type SQLJobRepository struct {
	db     *sql.DB
	goquDb *goqu.Database
}

func NewSQLJobRepository(db *sql.DB) *SQLJobRepository {
	goquDb := goqu.New("postgres", db)
	return &SQLJobRepository{db: db, goquDb: goquDb}
}

func (r *SQLJobRepository) GetQueueStats() ([]*lookout.QueueInfo, error) {
	rows, err := r.db.Query(`
		SELECT job.queue as queue, 
		       count(*) as jobs,
		       count(coalesce(job_run.created, job_run.started)) as jobs_created,
			   count(job_run.started) as Jobs_started
		FROM job LEFT JOIN job_run ON job.job_id = job_run.job_id
		WHERE job_run.finished IS NULL
		GROUP BY job.queue`)
	if err != nil {
		return nil, err
	}
	var (
		queue                          string
		jobs, jobsCreated, jobsStarted uint32
	)

	result := []*lookout.QueueInfo{}
	for rows.Next() {
		err := rows.Scan(&queue, &jobs, &jobsCreated, &jobsStarted)
		if err != nil {
			return nil, err
		}
		result = append(result, &lookout.QueueInfo{
			Queue:       queue,
			JobsQueued:  jobs - jobsCreated,
			JobsPending: jobsCreated - jobsStarted,
			JobsRunning: jobsStarted,
		})
	}
	return result, nil
}

func (r *SQLJobRepository) GetJobsInQueue(opts *lookout.GetJobsInQueueRequest) ([]*lookout.JobInfo, error) {
	rows, err := r.queryJobsInQueue(opts)
	if err != nil {
		return nil, err
	}

	result := jobsInQueueRowsToResult(rows)
	return result, nil
}

type jobsInQueueRow struct {
	JobId     string          `db:"job_id"`
	Queue     string          `db:"queue"`
	Owner     string          `db:"owner"`
	JobSet    string          `db:"jobset"`
	Priority  sql.NullFloat64 `db:"priority"`
	Submitted pq.NullTime     `db:"submitted"`
	Cancelled pq.NullTime     `db:"cancelled"`
	JobJson   sql.NullString  `db:"job"`
	RunId     sql.NullString  `db:"run_id"`
	Cluster   sql.NullString  `db:"cluster"`
	Node      sql.NullString  `db:"node"`
	Created   pq.NullTime     `db:"created"`
	Started   pq.NullTime     `db:"started"`
	Finished  pq.NullTime     `db:"finished"`
	Succeeded sql.NullBool    `db:"succeeded"`
	Error     sql.NullString  `db:"error"`
}

func (r *SQLJobRepository) queryJobsInQueue(opts *lookout.GetJobsInQueueRequest) ([]*jobsInQueueRow, error) {
	ds := r.createGetJobsInQueueDataset(opts)

	joinedRows := make([]*jobsInQueueRow, 0)
	err := ds.Prepared(true).ScanStructs(&joinedRows)
	if err != nil {
		return nil, err
	}

	return joinedRows, nil
}

func (r *SQLJobRepository) createGetJobsInQueueDataset(opts *lookout.GetJobsInQueueRequest) *goqu.SelectDataset {
	subDs := r.goquDb.
		From("job").
		LeftJoin(goqu.T("job_run"), goqu.On(goqu.Ex{
			"job.job_id": goqu.I("job_run.job_id"),
		})).
		Select(goqu.I("job.job_id")).
		Where(goqu.And(
			goqu.I("job.queue").Eq(opts.Queue),
			goqu.Or(createJobSetFilters(opts.JobSetIds)...))).
		GroupBy(goqu.I("job.job_id")).
		Having(goqu.Or(createJobStateFilters(opts.JobStates)...)).
		Order(createJobOrdering(opts.NewestFirst)).
		Limit(uint(opts.Take)).
		Offset(uint(opts.Skip))

	ds := r.goquDb.
		From("job").
		LeftJoin(goqu.T("job_run"), goqu.On(goqu.Ex{
			"job.job_id": goqu.I("job_run.job_id"),
		})).
		Select(
			goqu.I("job.job_id"),
			goqu.I("job.owner"),
			goqu.I("job.jobset"),
			goqu.I("job.priority"),
			goqu.I("job.submitted"),
			goqu.I("job.cancelled"),
			goqu.I("job.job"),
			goqu.I("job_run.run_id"),
			goqu.I("job_run.cluster"),
			goqu.I("job_run.node"),
			goqu.I("job_run.created"),
			goqu.I("job_run.started"),
			goqu.I("job_run.finished"),
			goqu.I("job_run.succeeded"),
			goqu.I("job_run.error")).
		Where(goqu.I("job.job_id").In(subDs)).
		Order(createJobOrdering(opts.NewestFirst)) // Ordering from sub query not guaranteed to be preserved

	return ds
}

func createJobSetFilters(jobSetIds []string) []goqu.Expression {
	filters := make([]goqu.Expression, 0)
	for _, jobSetId := range jobSetIds {
		filter := goqu.I("job.jobset").Like(jobSetId + "%")
		filters = append(filters, filter)
	}
	return filters
}

const (
	submitted = "job.submitted"
	cancelled = "job.cancelled"
	created   = "job_run.created"
	started   = "job_run.started"
	finished  = "job_run.finished"
	succeeded = "job_run.succeeded"
)

var filtersForState = map[lookout.JobState][]goqu.Expression{
	lookout.JobState_QUEUED: {
		goqu.MAX(goqu.I(submitted)).IsNotNull(),
		goqu.MAX(goqu.I(cancelled)).IsNull(),
		goqu.MAX(goqu.I(created)).IsNull(),
		goqu.MAX(goqu.I(started)).IsNull(),
		goqu.MAX(goqu.I(finished)).IsNull(),
	},
	lookout.JobState_PENDING: {
		goqu.MAX(goqu.I(cancelled)).IsNull(),
		goqu.MAX(goqu.I(created)).IsNotNull(),
		goqu.MAX(goqu.I(started)).IsNull(),
		goqu.MAX(goqu.I(finished)).IsNull(),
	},
	lookout.JobState_RUNNING: {
		goqu.MAX(goqu.I(cancelled)).IsNull(),
		goqu.MAX(goqu.I(started)).IsNotNull(),
		goqu.MAX(goqu.I(finished)).IsNull(),
	},
	lookout.JobState_SUCCEEDED: {
		goqu.MAX(goqu.I(cancelled)).IsNull(),
		goqu.MAX(goqu.I(finished)).IsNotNull(),
		BOOL_OR(goqu.I(succeeded)).IsTrue(),
	},
	lookout.JobState_FAILED: {
		BOOL_OR(goqu.I(succeeded)).IsFalse(),
	},
	lookout.JobState_CANCELLED: {
		goqu.MAX(goqu.I(cancelled)).IsNotNull(),
	},
}

func createJobStateFilters(jobStates []lookout.JobState) []goqu.Expression {
	filters := make([]goqu.Expression, 0)
	for _, state := range jobStates {
		filter := goqu.And(filtersForState[state]...)
		filters = append(filters, filter)
	}
	return filters
}

func createJobOrdering(newestFirst bool) exp.OrderedExpression {
	jobId := goqu.I("job.job_id")
	if newestFirst {
		return jobId.Desc()
	}
	return jobId.Asc()
}

func jobsInQueueRowsToResult(rows []*jobsInQueueRow) []*lookout.JobInfo {
	result := make([]*lookout.JobInfo, 0)

	for i, row := range rows {
		if i == 0 || result[len(result)-1].Job.Id != row.JobId {
			result = append(result, &lookout.JobInfo{
				Job: &api.Job{
					Id:          row.JobId,
					JobSetId:    row.JobSet,
					Queue:       row.Queue,
					Namespace:   "",
					Labels:      nil,
					Annotations: nil,
					Owner:       row.Owner,
					Priority:    ParseNullFloat(row.Priority),
					PodSpec:     nil,
					Created:     ParseNullTimeDefault(row.Submitted), // Job submitted
				},
				Cancelled: ParseNullTime(row.Cancelled),
				Runs:      []*lookout.RunInfo{},
			})
		}

		if row.RunId.Valid {
			result[len(result)-1].Runs = append(result[len(result)-1].Runs, &lookout.RunInfo{
				K8SId:     ParseNullString(row.RunId),
				Cluster:   ParseNullString(row.Cluster),
				Node:      ParseNullString(row.Node),
				Succeeded: ParseNullBool(row.Succeeded),
				Error:     ParseNullString(row.Error),
				Created:   ParseNullTime(row.Created), // Pod created (Pending)
				Started:   ParseNullTime(row.Started), // Pod running
				Finished:  ParseNullTime(row.Finished),
			})
		}
	}
	return result
}
