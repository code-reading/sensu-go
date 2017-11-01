package useractions

import (
	"github.com/sensu/sensu-go/backend/authorization"
	"github.com/sensu/sensu-go/backend/store"
	"github.com/sensu/sensu-go/types"
	"golang.org/x/net/context"
)

// EventActions expose actions in which a viewer can perform.
type EventActions struct {
	Store   store.EventStore
	Policy  authorization.EventPolicy
	Context context.Context
}

// NewEventActions returns new EventActions
func NewEventActions(ctx context.Context, store store.EventStore) EventActions {
	return EventActions{
		Store:   store,
		Policy:  authorization.Events.WithContext(ctx),
		Context: ctx,
	}
}

// Query returns resources available to the viewer filter by given params.
func (a *EventActions) Query(params QueryParams) ([]interface{}, error) {
	var results []*types.Event

	entityID := params["entity"]
	checkName := params["check"]

	// Fetch from store
	var serr error
	if entityID != "" && checkName != "" {
		var result *types.Event
		result, serr = a.Store.GetEventByEntityCheck(a.Context, entityID, checkName)
		results = append(results, result)
	} else if entityID != "" {
		results, serr = a.Store.GetEventsByEntity(a.Context, entityID)
	} else {
		results, serr = a.Store.GetEvents(a.Context)
	}

	if serr != nil {
		return nil, NewError(InternalErr, serr)
	}

	// Filter out those resources the viewer does not have access to view.
	resources := []interface{}{}
	for _, event := range results {
		if yes := a.Policy.CanRead(event); yes {
			resources = append(resources, event)
		}
	}

	return resources, nil
}

// Find returns resource associated with given parameters if available to the
// viewer.
func (a *EventActions) Find(params QueryParams) (interface{}, error) {
	results, err := a.Query(params)

	var result interface{}
	if len(results) > 0 {
		result = results[0]
	}
	return result, err
}
