package resolver

import (
	"context"
	"fmt"
	"github.com/apito-io/engine/models"
)

type GraphQLSubscriptions struct {
	subscriber map[string]*models.Subscriber
}

func GetGraphQLSubscriptions() (*GraphQLSubscriptions, error) {
	sm := &GraphQLSubscriptions{
		subscriber: make(map[string]*models.Subscriber),
	}
	return sm, nil
}

// subscribe calls a subscription function from handler that has been passed through context
func (s *GraphQLSubscriptions) subscribe(ctx context.Context, userID string) (*models.Subscriber, error) {

	dataChannel := make(chan interface{}, 1)

	sub := &models.Subscriber{
		Data:   dataChannel,
		UserID: userID,
	}

	err := s.add(ctx, sub)
	if err != nil {
		return nil, err
	}

	// run a routine to listen on the context and remove the subscriber when it's done
	go func() {
		<-ctx.Done()
		err := s.remove(ctx, userID)
		if err != nil {
			return
		}
	}()

	fmt.Println(fmt.Sprintf("subscriber added, user id : %s", userID))

	return sub, nil
}

func (s *GraphQLSubscriptions) add(ctx context.Context, sub *models.Subscriber) error {
	s.subscriber[sub.UserID] = sub
	return nil
}

func (s *GraphQLSubscriptions) remove(ctx context.Context, userID string) error {
	if len(s.subscriber) > 0 {
		close(s.subscriber[userID].Data) // close the channel
		delete(s.subscriber, userID)     // delete from map
	}
	return nil
}

func (s *GraphQLSubscriptions) getSubscribers(ctx context.Context) map[string]*models.Subscriber {
	return s.subscriber
}
