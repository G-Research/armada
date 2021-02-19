// Code generated by statik. DO NOT EDIT.

package statik

import (
	"github.com/rakyll/statik/fs"
)

const LookoutSql = "lookout/sql" // static asset namespace

func init() {
	data := "PK\x03\x04\x14\x00\x08\x00\x00\x00\x00\x00!(\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x16\x00	\x00001_initial_schema.sqlUT\x05\x00\x01\x80Cm8CREATE TABLE job\n(\n    job_id    varchar(32)  NOT NULL PRIMARY KEY,\n    queue     varchar(512) NOT NULL,\n    owner     varchar(512) NULL,\n    jobset    varchar(512) NOT NULL,\n\n    priority  float        NULL,\n    submitted timestamp    NULL,\n    cancelled timestamp    NULL,\n\n    job       jsonb        NULL\n);\n\nCREATE TABLE job_run\n(\n    run_id    varchar(36)  NOT NULL PRIMARY KEY,\n    job_id    varchar(32)  NOT NULL,\n\n    cluster   varchar(512) NULL,\n    node      varchar(512) NULL,\n\n    created   timestamp    NULL,\n    started   timestamp    NULL,\n    finished  timestamp    NULL,\n\n    succeeded bool         NULL,\n    error     varchar(512) NULL\n);\n\nCREATE TABLE job_run_container\n(\n    run_id         varchar(32) NOT NULL,\n    container_name varchar(512) NOT NULL,\n    exit_code      int         NOT NULL,\n    PRIMARY KEY (run_id, container_name)\n)\n\n\nPK\x07\x08A\x9e\xa2$\\\x03\x00\x00\\\x03\x00\x00PK\x03\x04\x14\x00\x08\x00\x00\x00\x00\x00!(\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x1b\x00	\x00002_increase_error_size.sqlUT\x05\x00\x01\x80Cm8ALTER TABLE job_run ALTER COLUMN error TYPE varchar(2048);\nPK\x07\x08)\xc1\xe0\x87;\x00\x00\x00;\x00\x00\x00PK\x03\x04\x14\x00\x08\x00\x00\x00\x00\x00!(\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x17\x00	\x00003_fix_run_id_size.sqlUT\x05\x00\x01\x80Cm8ALTER TABLE job_run_container ALTER COLUMN run_id TYPE varchar(36);\nPK\x07\x08\x0cD$\xeaD\x00\x00\x00D\x00\x00\x00PK\x03\x04\x14\x00\x08\x00\x00\x00\x00\x00!(\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x0f\x00	\x00004_indexes.sqlUT\x05\x00\x01\x80Cm8-- jobs are looked up by queue, jobset\nCREATE INDEX idx_job_queue_jobset ON job(queue, jobset);\n\n-- ordering of jobs\nCREATE INDEX idx_job_submitted ON job(submitted);\n\n-- filtering of running jobs\nCREATE INDEX idx_jub_run_finished_null ON job_run(finished) WHERE finished IS NULL;\nPK\x07\x08\xa4#\xb1\xc8\x19\x01\x00\x00\x19\x01\x00\x00PK\x03\x04\x14\x00\x08\x00\x00\x00\x00\x00!(\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x16\x00	\x00005_multi_node_job.sqlUT\x05\x00\x01\x80Cm8ALTER TABLE Job_run ADD COLUMN pod_number int DEFAULT 0;\nPK\x07\x08\x18T,\xf19\x00\x00\x009\x00\x00\x00PK\x03\x04\x14\x00\x08\x00\x00\x00\x00\x00!(\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x1a\x00	\x00006_unable_to_schedule.sqlUT\x05\x00\x01\x80Cm8ALTER TABLE job_run ADD COLUMN unable_to_schedule bool NULL;\n\nCREATE INDEX idx_job_run_unable_to_schedule_null ON job_run(unable_to_schedule) WHERE unable_to_schedule IS NULL;\nPK\x07\x08\x0b\xdb~\xb3\xb0\x00\x00\x00\xb0\x00\x00\x00PK\x03\x04\x14\x00\x08\x00\x00\x00\x00\x00!(\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x12\x00	\x00007_job_states.sqlUT\x05\x00\x01\x80Cm8ALTER TABLE job ADD COLUMN state smallint NULL;\n\nCREATE INDEX idx_job_run_job_id ON job_run(job_id);\n\nCREATE INDEX idx_job_queue_state ON job(queue, state);\n\nCREATE INDEX idx_job_queue_jobset_state ON job(queue, jobset, state);\nPK\x07\x08\xa5\xe0*\x07\xe4\x00\x00\x00\xe4\x00\x00\x00PK\x01\x02\x14\x03\x14\x00\x08\x00\x00\x00\x00\x00!(A\x9e\xa2$\\\x03\x00\x00\\\x03\x00\x00\x16\x00	\x00\x00\x00\x00\x00\x00\x00\x00\x00\xa4\x81\x00\x00\x00\x00001_initial_schema.sqlUT\x05\x00\x01\x80Cm8PK\x01\x02\x14\x03\x14\x00\x08\x00\x00\x00\x00\x00!()\xc1\xe0\x87;\x00\x00\x00;\x00\x00\x00\x1b\x00	\x00\x00\x00\x00\x00\x00\x00\x00\x00\xa4\x81\xa9\x03\x00\x00002_increase_error_size.sqlUT\x05\x00\x01\x80Cm8PK\x01\x02\x14\x03\x14\x00\x08\x00\x00\x00\x00\x00!(\x0cD$\xeaD\x00\x00\x00D\x00\x00\x00\x17\x00	\x00\x00\x00\x00\x00\x00\x00\x00\x00\xa4\x816\x04\x00\x00003_fix_run_id_size.sqlUT\x05\x00\x01\x80Cm8PK\x01\x02\x14\x03\x14\x00\x08\x00\x00\x00\x00\x00!(\xa4#\xb1\xc8\x19\x01\x00\x00\x19\x01\x00\x00\x0f\x00	\x00\x00\x00\x00\x00\x00\x00\x00\x00\xa4\x81\xc8\x04\x00\x00004_indexes.sqlUT\x05\x00\x01\x80Cm8PK\x01\x02\x14\x03\x14\x00\x08\x00\x00\x00\x00\x00!(\x18T,\xf19\x00\x00\x009\x00\x00\x00\x16\x00	\x00\x00\x00\x00\x00\x00\x00\x00\x00\xa4\x81'\x06\x00\x00005_multi_node_job.sqlUT\x05\x00\x01\x80Cm8PK\x01\x02\x14\x03\x14\x00\x08\x00\x00\x00\x00\x00!(\x0b\xdb~\xb3\xb0\x00\x00\x00\xb0\x00\x00\x00\x1a\x00	\x00\x00\x00\x00\x00\x00\x00\x00\x00\xa4\x81\xad\x06\x00\x00006_unable_to_schedule.sqlUT\x05\x00\x01\x80Cm8PK\x01\x02\x14\x03\x14\x00\x08\x00\x00\x00\x00\x00!(\xa5\xe0*\x07\xe4\x00\x00\x00\xe4\x00\x00\x00\x12\x00	\x00\x00\x00\x00\x00\x00\x00\x00\x00\xb4\x81\xae\x07\x00\x00007_job_states.sqlUT\x05\x00\x01\x80Cm8PK\x05\x06\x00\x00\x00\x00\x07\x00\x07\x00\x1a\x02\x00\x00\xdb\x08\x00\x00\x00\x00"
	fs.RegisterWithNamespace("lookout/sql", data)
}
