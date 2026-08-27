package notification

import (
	"context"
	"fmt"
)

type Sender interface {
	Send(context.Context, Notification) error
}
type Worker struct {
	service *Service
	sender  Sender
}

func NewWorker(service *Service, sender Sender) *Worker {
	return &Worker{service: service, sender: sender}
}
func (w *Worker) RunBatch(ctx context.Context, limit int) (int, error) {
	items, err := w.service.Claim(ctx, limit)
	if err != nil {
		return 0, err
	}
	for _, item := range items {
		if err = w.sender.Send(ctx, item); err != nil {
			if markErr := w.service.MarkFailed(ctx, item.ID, err.Error(), item.Attempts); markErr != nil {
				return 0, fmt.Errorf("mark notification failed: %w", markErr)
			}
			continue
		}
		if err = w.service.MarkSent(ctx, item.ID); err != nil {
			return 0, err
		}
	}
	return len(items), nil
}
