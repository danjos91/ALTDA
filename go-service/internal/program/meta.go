package program

// CollectLockedIDs returns node ids that have _meta.locked == true.
func CollectLockedIDs(node any) map[string]bool {
	locked := map[string]bool{}
	walkLocked(node, locked)
	return locked
}

func walkLocked(node any, locked map[string]bool) {
	switch n := node.(type) {
	case Object:
		walkLockedMap(n, locked)
	case map[string]any:
		walkLockedMap(Object(n), locked)
	case Array:
		for _, child := range n {
			walkLocked(child, locked)
		}
	case []any:
		for _, child := range n {
			walkLocked(child, locked)
		}
	}
}

func walkLockedMap(n Object, locked map[string]bool) {
	if meta, ok := AsObject(n["_meta"]); ok && GetBool(meta, "locked", false) {
		if id := GetString(n, "id", ""); id != "" {
			locked[id] = true
		}
	}
	for _, child := range n {
		walkLocked(child, locked)
	}
}

// CollectDependents returns nodeIds plus any nodes that list them in _meta.dependsOn.
func CollectDependents(program Object, nodeIDs []string) map[string]bool {
	target := map[string]bool{}
	for _, id := range nodeIDs {
		target[id] = true
	}
	changed := true
	for changed {
		changed = false
		walkDepends(program, func(id string, depends []string) {
			if target[id] {
				return
			}
			for _, d := range depends {
				if target[d] {
					target[id] = true
					changed = true
					return
				}
			}
		})
	}
	return target
}

func walkDepends(node any, fn func(id string, depends []string)) {
	switch n := node.(type) {
	case Object:
		walkDependsMap(n, fn)
	case map[string]any:
		walkDependsMap(Object(n), fn)
	case Array:
		for _, child := range n {
			walkDepends(child, fn)
		}
	case []any:
		for _, child := range n {
			walkDepends(child, fn)
		}
	}
}

func walkDependsMap(n Object, fn func(id string, depends []string)) {
	id := GetString(n, "id", "")
	var depends []string
	if meta, ok := AsObject(n["_meta"]); ok {
		if arr, ok := AsArray(meta["dependsOn"]); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok && s != "" {
					depends = append(depends, s)
				}
			}
		}
	}
	if id != "" {
		fn(id, depends)
	}
	for _, child := range n {
		walkDepends(child, fn)
	}
}

// FindNodeByID finds the first object with the given id.
func FindNodeByID(node any, id string) Object {
	var found Object
	walkFind(node, id, &found)
	return found
}

func walkFind(node any, id string, found *Object) {
	if *found != nil {
		return
	}
	switch n := node.(type) {
	case Object:
		if GetString(n, "id", "") == id {
			*found = n
			return
		}
		for _, child := range n {
			walkFind(child, id, found)
		}
	case map[string]any:
		walkFind(Object(n), id, found)
	case Array:
		for _, child := range n {
			walkFind(child, id, found)
		}
	case []any:
		for _, child := range n {
			walkFind(child, id, found)
		}
	}
}

// TotalHoursFromProgram resolves requested total hours from header or request.params.
func TotalHoursFromProgram(program Object) float64 {
	if h := Nested(program, "header"); h != nil {
		if v := GetFloat(h, "totalHours", 0); v > 0 {
			return v
		}
	}
	if p := Nested(program, "request", "params"); p != nil {
		if v := GetFloat(p, "totalHours", 0); v > 0 {
			return v
		}
	}
	return 16
}

// IsNodeLocked reports whether the node with id is locked.
func IsNodeLocked(program Object, id string) bool {
	n := FindNodeByID(program, id)
	if n == nil {
		return false
	}
	meta, ok := AsObject(n["_meta"])
	return ok && GetBool(meta, "locked", false)
}
