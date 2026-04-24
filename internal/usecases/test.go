package usecases

import (
	"context"
	"room-booking/internal/repository"
)

type TestUseCase interface {
	Save(ctx context.Context, msg string) error
}

type testUC struct {
	repo repository.TestRepository
}

func NewTestUseCase(repo repository.TestRepository) TestUseCase {
	return &testUC{repo: repo}
}

func (uc *testUC) Save(ctx context.Context, msg string) error {
	return uc.repo.SaveMessage(ctx, msg)
}
