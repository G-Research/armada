// Code generated by statik. DO NOT EDIT.

package statik

import (
	"github.com/rakyll/statik/fs"
)

const LookoutSql = "lookout/sql" // static asset namespace

func init() {
	data := "PK\x03\x04\x14\x00\x08\x00\x00\x00\xa0\x8cmQ\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x16\x00	\x00001_initial_schema.sqlUT\x05\x00\x01=\xc4\xae_CREATE TABLE job\n(\n    job_id    varchar(32)  NOT NULL PRIMARY KEY,\n    queue     varchar(512) NOT NULL,\n    owner     varchar(512) NULL,\n    jobset    varchar(512) NOT NULL,\n\n    priority  float        NULL,\n    submitted timestamp    NULL,\n    cancelled timestamp    NULL,\n\n    job       jsonb        NULL\n);\n\nCREATE TABLE job_run\n(\n    run_id    varchar(36)  NOT NULL PRIMARY KEY,\n    job_id    varchar(32)  NOT NULL,\n\n    cluster   varchar(512) NULL,\n    node      varchar(512) NULL,\n\n    created   timestamp    NULL,\n    started   timestamp    NULL,\n    finished  timestamp    NULL,\n\n    succeeded bool         NULL,\n    error     varchar(512) NULL\n);\n\nCREATE TABLE job_run_container\n(\n    run_id         varchar(32) NOT NULL,\n    container_name varchar(512) NOT NULL,\n    exit_code      int         NOT NULL,\n    PRIMARY KEY (run_id, container_name)\n)\n\n\nPK\x07\x08A\x9e\xa2$\\\x03\x00\x00\\\x03\x00\x00PK\x03\x04\x14\x00\x08\x00\x00\x00u_\x84Q\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x1b\x00	\x00002_increase_error_size.sqlUT\x05\x00\x01\xae$\xca_ALTER TABLE job_run ALTER COLUMN error TYPE varchar(2048);\nPK\x07\x08)\xc1\xe0\x87;\x00\x00\x00;\x00\x00\x00PK\x03\x04\x14\x00\x08\x00\x00\x00\x00`\x84Q\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x17\x00	\x00003_fix_run_id_size.sqlUT\x05\x00\x01\xc1$\xca_ALTER TABLE job_run_container ALTER COLUMN run_id TYPE varchar(36);\nPK\x07\x08\x0cD$\xeaD\x00\x00\x00D\x00\x00\x00PK\x01\x02\x14\x03\x14\x00\x08\x00\x00\x00\xa0\x8cmQA\x9e\xa2$\\\x03\x00\x00\\\x03\x00\x00\x16\x00	\x00\x00\x00\x00\x00\x00\x00\x00\x00\xa4\x81\x00\x00\x00\x00001_initial_schema.sqlUT\x05\x00\x01=\xc4\xae_PK\x01\x02\x14\x03\x14\x00\x08\x00\x00\x00u_\x84Q)\xc1\xe0\x87;\x00\x00\x00;\x00\x00\x00\x1b\x00	\x00\x00\x00\x00\x00\x00\x00\x00\x00\xa4\x81\xa9\x03\x00\x00002_increase_error_size.sqlUT\x05\x00\x01\xae$\xca_PK\x01\x02\x14\x03\x14\x00\x08\x00\x00\x00\x00`\x84Q\x0cD$\xeaD\x00\x00\x00D\x00\x00\x00\x17\x00	\x00\x00\x00\x00\x00\x00\x00\x00\x00\xa4\x816\x04\x00\x00003_fix_run_id_size.sqlUT\x05\x00\x01\xc1$\xca_PK\x05\x06\x00\x00\x00\x00\x03\x00\x03\x00\xed\x00\x00\x00\xc8\x04\x00\x00\x00\x00"
	fs.RegisterWithNamespace("lookout/sql", data)
}