"""Shared in-memory JSON mutation support for offline dynamic-volume checks."""

import copy

DELETE = object()


def mutate_json(data, path, value):
    changed = copy.deepcopy(data)
    parent = changed
    for key in path[:-1]:
        parent = parent[key]
    if value is DELETE:
        del parent[path[-1]]
    else:
        parent[path[-1]] = value
    return changed
