package service

import (
	"context"

	"github.com/google/uuid"

	"nowdone/internal/models"
	"nowdone/internal/repository"
)

// TaskTypeService implements CRUD for user-defined task categories.
type TaskTypeService struct {
	taskTypes *repository.TaskTypeRepository
}

func NewTaskTypeService(taskTypes *repository.TaskTypeRepository) *TaskTypeService {
	return &TaskTypeService{taskTypes: taskTypes}
}

func (s *TaskTypeService) List(ctx context.Context, userID uuid.UUID) ([]*models.TaskType, error) {
	return s.taskTypes.ListByUser(ctx, userID)
}

func (s *TaskTypeService) Create(ctx context.Context, userID uuid.UUID, emoji, name string) (*models.TaskType, error) {
	return s.taskTypes.Create(ctx, userID, emoji, name)
}

func (s *TaskTypeService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.taskTypes.Delete(ctx, userID, id)
}
