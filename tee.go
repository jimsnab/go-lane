package lane

import "sync"

var teeMu sync.Mutex

func addTee(source, target Lane, appendTee func()) {
	teeMu.Lock()
	defer teeMu.Unlock()

	if reachesLane(target, source.LaneId(), map[string]bool{}) {
		panic("tee would create a cycle")
	}
	appendTee()
}

func reachesLane(l Lane, laneID string, visited map[string]bool) bool {
	id := l.LaneId()
	if id == laneID {
		return true
	}
	if visited[id] {
		return false
	}
	visited[id] = true

	for _, tee := range l.Tees() {
		if reachesLane(tee, laneID, visited) {
			return true
		}
	}
	return false
}