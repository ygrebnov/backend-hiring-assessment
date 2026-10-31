package identity

import (
	"context"
	"time"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	userspkg "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersapi "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

type Adapter struct {
	client *usersapi.Client
}

var _ userspkg.UserClient = (*Adapter)(nil)

func New(baseURL string, timeout time.Duration) *Adapter {
	return &Adapter{
		client: usersapi.NewClient(baseURL, timeout),
	}
}

func (a *Adapter) GetUserByID(ctx context.Context, userID model.UserID) (userspkg.User, error) {
	user, err := a.client.GetUserByID(ctx, string(userID))
	if err != nil {
		return userspkg.User{}, err
	}

	return userspkg.User{
		UserID:   user.UserID,
		Nickname: user.Nickname,
	}, nil
}

func (a *Adapter) ListUsers(ctx context.Context) ([]userspkg.User, error) {
	users, err := a.client.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]userspkg.User, len(users))
	for i, user := range users {
		result[i] = userspkg.User{
			UserID:   user.UserID,
			Nickname: user.Nickname,
		}
	}

	return result, nil
}
