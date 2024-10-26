// clang-format off
//go:build !nowatcher
// clang-format on
#include "_cgo_export.h"
#include "wtr/watcher-c.h"

void handle_event(struct wtr_watcher_event event, void *data) {
  go_handle_file_watcher_event((char *)event.path_name, event.effect_type,
                               event.path_type, (uintptr_t)data);
}

uintptr_t start_new_watcher(char const *const path, uintptr_t data) {
  void *watcher = wtr_watcher_open(path, handle_event, (void *)data);
  if (watcher == NULL) {
    return 0;
  }
  return (uintptr_t)watcher;
}

int stop_watcher(uintptr_t watcher) {
  if (!wtr_watcher_close((void *)watcher)) {
    return 0;
  }
  return 1;
}
