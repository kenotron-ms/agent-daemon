    //go:build darwin && cgo

    package meeting

    import "C"

    import "sync"

    // notifCallbacks maps notification ID → action callback.
    var (
    	notifMu        sync.Mutex
    	notifCallbacks = map[string]func(string){}
    )

    //export notifGoAction
    func notifGoAction(notifID *C.char, actionID *C.char) {
    	id := C.GoString(notifID)
    	action := C.GoString(actionID)

    	notifMu.Lock()
    	cb, ok := notifCallbacks[id]
    	if ok {
    		delete(notifCallbacks, id)
    	}
    	notifMu.Unlock()

    	if ok && cb != nil {
    		cb(action)
    	}
    }
    