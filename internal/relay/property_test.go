//go:build property

package relay

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

// TestPropertyPlanRepeatsNothingOnceCarriedOut is the restart property of the
// bridge's bookkeeping: whatever the forge holds and whatever the operator has
// said, carrying out a plan's actions leaves a state that plans nothing more.
func TestPropertyPlanRepeatsNothingOnceCarriedOut(t *testing.T) {
	property := func(st State, requests []Request, events []RoomEvent) bool {
		st = cloneState(st)
		requests = append([]Request(nil), requests...)

		carryOut(&st, &requests, Plan(operator, st, requests, events))

		return len(Plan(operator, st, requests, events)) == 0
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomState(rnd))
			values[1] = reflect.ValueOf(randomRequests(rnd))
			values[2] = reflect.ValueOf(randomEvents(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyPlanIgnoresStrangersMessages bounds the allowlist: an event from
// anyone but the operator never becomes a chat request, whatever it says.
func TestPropertyPlanIgnoresStrangersMessages(t *testing.T) {
	property := func(event RoomEvent) bool {
		event.Sender = "@stranger:example.org"
		for _, action := range Plan(operator, State{}, nil, []RoomEvent{event}) {
			if action.Kind == CreateForgeRequest {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

// TestPropertyPlanAsksForEachEligibleMessageOnce is the conservation rule: a
// plan asks the forge to create a chat request for every operator message the
// forge has not seen, and for nothing else.
func TestPropertyPlanAsksForEachEligibleMessageOnce(t *testing.T) {
	property := func(st State, events []RoomEvent) bool {
		st = cloneState(st)

		want := make(map[string]int)
		for _, event := range events {
			if operatorAsked(st, operator, event) {
				want[event.EventID]++
			}
		}

		got := make(map[string]int)
		for _, action := range Plan(operator, st, nil, events) {
			if action.Kind != CreateForgeRequest {
				return false
			}
			got[action.SourceEventID]++
		}

		return reflect.DeepEqual(got, want)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomState(rnd))
			values[1] = reflect.ValueOf(randomEvents(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyPlanPostsARequestBeforeAnsweringIt is the ordering rule the
// thread needs: an answer is threaded under the message the same plan posts, so
// that message comes first. It holds for the states the bridge can reach: the
// Matrix client drops an event without an id, so a relayed message always names
// a non-empty event. (A state that broke that would make the plan ask for a
// reply with no anchor, which the bridge refuses to carry out.)
func TestPropertyPlanPostsARequestBeforeAnsweringIt(t *testing.T) {
	property := func(st State, requests []Request) bool {
		st = cloneState(st)
		posted := make(map[string]bool)

		for _, action := range Plan(operator, st, requests, nil) {
			switch action.Kind {
			case PostRequestMessage:
				posted[action.RequestID] = true
			case PostRequestReply:
				if action.AnchorEventID == "" && !posted[action.RequestID] {
					return false
				}
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomState(rnd))
			values[1] = reflect.ValueOf(randomRequests(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// carryOut records the effects a plan's actions have on the bridge's state and
// on the forge, the way a tick does.
func carryOut(st *State, requests *[]Request, actions []Action) {
	st.EnsureMaps()
	for _, action := range actions {
		switch action.Kind {
		case PostRequestMessage:
			st.Threads[action.RequestID] = "$posted-" + action.RequestID
		case PostRequestReply:
			st.Replied[action.RequestID] = "$replied-" + action.RequestID
		case CreateForgeRequest:
			requestID := "$request-" + action.SourceEventID
			st.Relayed[action.SourceEventID] = requestID
			st.Threads[requestID] = action.SourceEventID
			*requests = append(*requests, Request{ID: requestID, Body: action.Body})
		}
	}
}

func cloneState(st State) State {
	cloned := State{
		Threads: map[string]string{},
		Replied: map[string]string{},
		Relayed: map[string]string{},
	}
	for key, value := range st.Threads {
		cloned.Threads[key] = value
	}
	for key, value := range st.Replied {
		cloned.Replied[key] = value
	}
	for key, value := range st.Relayed {
		cloned.Relayed[key] = value
	}
	return cloned
}

func randomState(rnd *rand.Rand) State {
	return State{
		Threads: randomMap(rnd),
		Replied: randomMap(rnd),
		// Only a message with an id is relayed: the client drops the rest.
		Relayed: randomMapWithKeyTokens(rnd, []string{"$event-1", "$event-2", "req-2"}),
	}
}

func randomMap(rnd *rand.Rand) map[string]string {
	return randomMapWithKeyTokens(rnd, randomTokens)
}

// randomMapWithKeyTokens builds a lookup whose keys come from a set of tokens.
func randomMapWithKeyTokens(rnd *rand.Rand, keyTokens []string) map[string]string {
	entries := map[string]string{}
	for count := rnd.Intn(4); count > 0; count-- {
		entries[keyTokens[rnd.Intn(len(keyTokens))]] = randomToken(rnd)
	}
	if len(entries) == 0 {
		return nil
	}
	return entries
}

func randomRequests(rnd *rand.Rand) []Request {
	var requests []Request
	for count := rnd.Intn(4); count > 0; count-- {
		request := Request{ID: randomToken(rnd), Body: randomText(rnd)}
		if rnd.Intn(2) == 0 {
			request.Response = randomText(rnd)
		}
		requests = append(requests, request)
	}
	return requests
}

func randomEvents(rnd *rand.Rand) []RoomEvent {
	var events []RoomEvent
	for count := rnd.Intn(4); count > 0; count-- {
		sender := operator
		switch rnd.Intn(3) {
		case 0:
			sender = "@stranger:example.org"
		case 1:
			sender = ""
		}
		events = append(events, RoomEvent{
			RoomID:  "!room:example.org",
			EventID: randomToken(rnd),
			Sender:  sender,
			Body:    randomText(rnd),
		})
	}
	return events
}

// randomTokens are the ids and texts a state and a room can hold. It holds the
// empty string because a request id the dashboard never wrote, or an anchor the
// bridge never recorded, reads as empty.
var randomTokens = []string{"", "req-1", "req-2", "$event-1", "$event-2", "text"}

func randomToken(rnd *rand.Rand) string {
	return randomTokens[rnd.Intn(len(randomTokens))]
}

func randomText(rnd *rand.Rand) string {
	texts := []string{"", " ", "is the build green?", "yes, the build is green", "two  spaces", "$event-1"}
	return texts[rnd.Intn(len(texts))]
}
