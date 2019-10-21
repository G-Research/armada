package server

import (
	"context"

	"github.com/gogo/protobuf/types"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/G-Research/armada/internal/armada/api"
	"github.com/G-Research/armada/internal/armada/authorization"
	"github.com/G-Research/armada/internal/armada/repository"
)

type SubmitServer struct {
	permissions     authorization.PermissionChecker
	jobRepository   repository.JobRepository
	queueRepository repository.QueueRepository
	eventRepository repository.EventRepository
}

func NewSubmitServer(
	permissions authorization.PermissionChecker,
	jobRepository repository.JobRepository,
	queueRepository repository.QueueRepository,
	eventRepository repository.EventRepository) *SubmitServer {

	return &SubmitServer{
		permissions:     permissions,
		jobRepository:   jobRepository,
		queueRepository: queueRepository,
		eventRepository: eventRepository}
}

func (server *SubmitServer) CreateQueue(ctx context.Context, queue *api.Queue) (*types.Empty, error) {
	if e := checkPermission(server.permissions, ctx, authorization.CreateQueue); e != nil {
		return nil, e
	}

	e := server.queueRepository.CreateQueue(queue)
	if e != nil {
		return nil, status.Errorf(codes.Aborted, e.Error())
	}
	return &types.Empty{}, nil
}

func (server *SubmitServer) SubmitJob(ctx context.Context, req *api.JobRequest) (*api.JobSubmitResponse, error) {
	if e := checkPermission(server.permissions, ctx, authorization.SubmitJobs); e != nil {
		return nil, e
	}

	job := server.jobRepository.CreateJob(req)

	e := reportSubmitted(server.eventRepository, job)
	if e != nil {
		return nil, status.Errorf(codes.Aborted, e.Error())
	}

	e = server.jobRepository.AddJob(job)
	if e != nil {
		return nil, status.Errorf(codes.Aborted, e.Error())
	}
	result := &api.JobSubmitResponse{JobId: job.Id}

	e = reportQueued(server.eventRepository, job)
	if e != nil {
		return result, status.Errorf(codes.Aborted, e.Error())
	}

	return result, nil
}

func (server *SubmitServer) CancelJobs(ctx context.Context, request *api.JobCancelRequest) (*api.CancellationResult, error) {
	if e := checkPermission(server.permissions, ctx, authorization.CancelJobs); e != nil {
		return nil, e
	}

	if request.JobId != "" {
		return server.cancelJobs([]string{request.JobId})
	}

	if request.JobSetId != "" && request.Queue != "" {
		ids, e := server.jobRepository.GetActiveJobIds(request.Queue, request.JobSetId)
		if e != nil {
			return nil, status.Errorf(codes.Aborted, e.Error())
		}
		return server.cancelJobs(ids)
	}
	return nil, status.Errorf(codes.InvalidArgument, "Specify job id or queue with job set id")
}

func (server *SubmitServer) cancelJobs(ids []string) (*api.CancellationResult, error) {
	jobs, e := server.jobRepository.GetJobsByIds(ids)
	if e != nil {
		return nil, status.Errorf(codes.Internal, e.Error())
	}

	e = reportJobsCancelling(server.eventRepository, jobs)
	if e != nil {
		return nil, status.Errorf(codes.Unknown, e.Error())
	}

	cancellationResult := server.jobRepository.Cancel(jobs)
	cancelled := []*api.Job{}
	cancelledIds := []string{}
	for job, error := range cancellationResult {
		if error != nil {
			log.Errorf("Error when cancelling job id %s: %s", job.Id, error.Error())
		} else {
			cancelled = append(cancelled, job)
			cancelledIds = append(cancelledIds, job.Id)
		}
	}

	e = reportJobsCancelled(server.eventRepository, cancelled)
	if e != nil {
		return nil, status.Errorf(codes.Unknown, e.Error())
	}

	return &api.CancellationResult{cancelledIds}, nil
}
