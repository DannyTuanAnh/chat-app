package ws

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// LƯU Ý: ĐÂY LÀ CODE ĐỂ TEST BENCHMARK
type fakeConn struct {
	writes atomic.Int64

	delay time.Duration

	completedAt atomic.Int64

	wg *sync.WaitGroup
}

func newFakeConn(delay time.Duration) *fakeConn {
	return &fakeConn{
		delay: delay,
	}
}

func (c *fakeConn) Write(
	ctx context.Context,
	typ websocket.MessageType,
	data []byte,
) error {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}

	c.writes.Add(1)

	c.completedAt.Store(time.Now().UnixNano())

	if c.wg != nil {
		c.wg.Done()
	}

	return nil
}

func (c *fakeConn) Reset() {
	c.writes.Store(0)
}

func TestBOld_ManySlowClients(t *testing.T) {
	const (
		numFast = 10
		numSlow = 10
	)

	total := numFast + numSlow

	var wg sync.WaitGroup
	wg.Add(total)

	manager := NewOldClientManager()

	fastClients := make([]*fakeConn, 0, numFast)
	slowClients := make([]*fakeConn, 0, numSlow)
	userIDs := make([]int32, 0, total)

	var userID int32 = 1

	// Interleave fast / slow
	for i := 0; i < numSlow; i++ {
		// FAST
		fast := newFakeConn(0)
		fast.wg = &wg

		manager.AddClient(&Client{
			UserID:   userID,
			DeviceID: fmt.Sprintf("device-%d", userID),
			Conn:     fast,
		})

		fastClients = append(fastClients, fast)
		userIDs = append(userIDs, userID)
		userID++

		// SLOW
		slow := newFakeConn(100 * time.Millisecond)
		slow.wg = &wg

		manager.AddClient(&Client{
			UserID:   userID,
			DeviceID: fmt.Sprintf("device-%d", userID),
			Conn:     slow,
		})

		slowClients = append(slowClients, slow)
		userIDs = append(userIDs, userID)
		userID++
	}

	data := Content{
		FromUserID: 1,
		Message:    "hello",
	}

	start := time.Now()

	err := manager.SendToUsers(
		context.Background(),
		OldSendToUsersParams{
			UserIDs:     userIDs,
			MessageType: websocket.MessageText,
			Data:        data,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	dispatch := time.Since(start)

	wg.Wait()

	totalElapsed := time.Since(start)

	t.Logf("OLD")
	t.Logf("dispatch: %v", dispatch)
	t.Logf("total: %v", totalElapsed)

	for i, client := range fastClients {
		completed := time.Unix(
			0,
			client.completedAt.Load(),
		)

		t.Logf(
			"fast[%d] completed: %v",
			i,
			completed.Sub(start),
		)
	}

	for i, client := range slowClients {
		completed := time.Unix(
			0,
			client.completedAt.Load(),
		)

		t.Logf(
			"slow[%d] completed: %v",
			i,
			completed.Sub(start),
		)
	}
}

func TestBNew_ManySlowClients(t *testing.T) {
	const (
		numFast = 10
		numSlow = 10
	)

	total := numFast + numSlow

	manager := NewNewManager()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(total)

	fastClients := make([]*fakeConn, 0, numFast)
	slowClients := make([]*fakeConn, 0, numSlow)
	userIDs := make([]int32, 0, total)

	var userID int32 = 1

	addClient := func(
		conn *fakeConn,
		id int32,
	) {
		conn.wg = &wg

		clientCtx, clientCancel := context.WithCancel(ctx)

		client := &Client{
			UserID:   id,
			DeviceID: fmt.Sprintf("device-%d", id),
			Conn:     conn,

			SendChan: make(chan OutboundMessage, 256),
			ErrChan:  make(chan error, 1),

			Ctx:    clientCtx,
			Cancel: clientCancel,
		}

		manager.AddClient(client)

		go client.WritePump()
	}

	// Interleave fast / slow
	for i := 0; i < numSlow; i++ {
		fast := newFakeConn(0)

		addClient(fast, userID)

		fastClients = append(fastClients, fast)
		userIDs = append(userIDs, userID)

		userID++

		slow := newFakeConn(100 * time.Millisecond)

		addClient(slow, userID)

		slowClients = append(slowClients, slow)
		userIDs = append(userIDs, userID)

		userID++
	}

	data := Content{
		FromUserID: 1,
		Message:    "hello",
	}

	start := time.Now()

	err := manager.SendToUsers(
		context.Background(),
		NewSendToUsersParams{
			UserIDs:     userIDs,
			MessageType: websocket.MessageText,
			Data:        data,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	dispatch := time.Since(start)

	// Chờ tất cả 20 writes hoàn thành.
	wg.Wait()

	totalElapsed := time.Since(start)

	t.Logf("NEW")
	t.Logf("dispatch: %v", dispatch)
	t.Logf("total: %v", totalElapsed)

	for i, client := range fastClients {
		completed := time.Unix(
			0,
			client.completedAt.Load(),
		)

		t.Logf(
			"fast[%d] completed: %v",
			i,
			completed.Sub(start),
		)
	}

	for i, client := range slowClients {
		completed := time.Unix(
			0,
			client.completedAt.Load(),
		)

		t.Logf(
			"slow[%d] completed: %v",
			i,
			completed.Sub(start),
		)
	}
}

func newBenchmarkClient(
	userID int32,
	deviceID string,
) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		UserID:   userID,
		DeviceID: deviceID,

		Conn: &fakeConn{},

		SendChan: make(chan OutboundMessage, 256),
		ErrChan:  make(chan error, 1),

		Ctx:    ctx,
		Cancel: cancel,
	}
}

func setupOldManager(numUsers int) *OldClientManager {
	manager := NewOldClientManager()

	for i := 1; i <= numUsers; i++ {
		client := newBenchmarkClient(
			int32(i),
			fmt.Sprintf("device-%d", i),
		)

		manager.AddClient(client)
	}

	return manager
}

func setupOldSlowClientManager() (
	*OldClientManager,
	*fakeConn,
	*fakeConn,
) {
	manager := NewOldClientManager()

	fastAfter := newFakeConn(0)
	slow := newFakeConn(100 * time.Millisecond)

	manager.AddClient(&Client{
		UserID:   49,
		DeviceID: "device-49",
		Conn:     fastAfter,
	})

	manager.AddClient(&Client{
		UserID:   50,
		DeviceID: "device-50",
		Conn:     slow,
	})

	manager.AddClient(&Client{
		UserID:   51,
		DeviceID: "device-51",
		Conn:     fastAfter,
	})

	return manager, slow, fastAfter
}

func setupNewManager(numUsers int) *NewManager {
	manager := NewNewManager()

	for i := 1; i <= numUsers; i++ {
		client := newBenchmarkClient(
			int32(i),
			fmt.Sprintf("device-%d", i),
		)

		manager.AddClient(client)
	}

	return manager
}

func setupNewManagerWithWriters(numUsers int) (*NewManager, context.CancelFunc) {
	manager := NewNewManager()

	ctx, cancel := context.WithCancel(context.Background())

	for i := 1; i <= numUsers; i++ {
		clientCtx, clientCancel := context.WithCancel(ctx)

		client := &Client{
			UserID:   int32(i),
			DeviceID: fmt.Sprintf("device-%d", i),

			Conn: &fakeConn{},

			SendChan: make(chan OutboundMessage, 256),
			ErrChan:  make(chan error, 1),

			Ctx:    clientCtx,
			Cancel: clientCancel,
		}

		manager.AddClient(client)

		go client.WritePump()
	}

	return manager, cancel
}

func TestOldSlowClient(t *testing.T) {
	manager := NewOldClientManager()

	fastBefore := &fakeConn{
		delay: 500 * time.Millisecond,
	}
	slow := &fakeConn{
		delay: 1000 * time.Millisecond,
	}
	fastAfter := &fakeConn{}

	manager.AddClient(&Client{
		UserID:   49,
		DeviceID: "49",
		Conn:     fastBefore,
	})

	manager.AddClient(&Client{
		UserID:   50,
		DeviceID: "50",
		Conn:     slow,
	})

	manager.AddClient(&Client{
		UserID:   51,
		DeviceID: "51",
		Conn:     fastAfter,
	})

	start := time.Now()

	data := Content{
		FromUserID: 1,
		Message:    "hello",
	}

	err := manager.SendToUsers(
		context.Background(),
		OldSendToUsersParams{
			UserIDs:     []int32{49, 50, 51},
			MessageType: websocket.MessageText,
			Data:        data,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	elapsed := time.Since(start)

	fastBeforeCompleted := time.Unix(0, fastBefore.completedAt.Load())
	slowCompleted := time.Unix(0, slow.completedAt.Load())
	fastAfterCompleted := time.Unix(0, fastAfter.completedAt.Load())

	t.Logf("total: %v", elapsed)
	t.Logf("fast before: %v", fastBeforeCompleted.Sub(start))
	t.Logf("slow: %v", slowCompleted.Sub(start))
	t.Logf("fast after: %v", fastAfterCompleted.Sub(start))
}

func TestNewSlowClient(t *testing.T) {
	manager := NewNewManager()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fastBefore := newFakeConn(0)
	slow := newFakeConn(100 * time.Millisecond)
	fastAfter := newFakeConn(0)

	addClient := func(
		userID int32,
		conn *fakeConn,
	) {
		clientCtx, clientCancel := context.WithCancel(ctx)

		client := &Client{
			UserID:   userID,
			DeviceID: fmt.Sprintf("%d", userID),
			Conn:     conn,

			SendChan: make(chan OutboundMessage, 256),
			ErrChan:  make(chan error, 1),

			Ctx:    clientCtx,
			Cancel: clientCancel,
		}

		manager.AddClient(client)

		go client.WritePump()
	}

	addClient(49, fastBefore)
	addClient(50, slow)
	addClient(51, fastAfter)

	data := Content{
		FromUserID: 1,
		Message:    "hello",
	}

	start := time.Now()

	err := manager.SendToUsers(
		context.Background(),
		NewSendToUsersParams{
			UserIDs:     []int32{49, 50, 51},
			MessageType: websocket.MessageText,
			Data:        data,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	dispatchElapsed := time.Since(start)

	// Cho WritePump một khoảng thời gian để process
	time.Sleep(150 * time.Millisecond)

	fastAfterCompleted :=
		time.Unix(0, fastAfter.completedAt.Load())

	slowCompleted :=
		time.Unix(0, slow.completedAt.Load())

	t.Logf("dispatch: %v", dispatchElapsed)
	t.Logf(
		"fast after: %v",
		fastAfterCompleted.Sub(start),
	)
	t.Logf(
		"slow: %v",
		slowCompleted.Sub(start),
	)
}

func BenchmarkOld_SendToUsers_1000(b *testing.B) {
	manager := setupOldManager(1000)

	ctx := context.Background()

	data := Content{
		FromUserID: 1,
		Message:    "hello",
	}

	userIDs := make([]int32, 1000)

	for i := range userIDs {
		userIDs[i] = int32(i + 1)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := manager.SendToUsers(
			ctx,
			OldSendToUsersParams{
				UserIDs:     userIDs,
				MessageType: websocket.MessageText,
				Data:        data,
			},
		)

		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNew_SendToUsers_1000(b *testing.B) {
	manager, cancel := setupNewManagerWithWriters(1000)
	defer cancel()

	ctx := context.Background()

	userIDs := make([]int32, 1000)

	for i := range userIDs {
		userIDs[i] = int32(i + 1)
	}

	data := Content{
		FromUserID: 1,
		Message:    "hello",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := manager.SendToUsers(
			ctx,
			NewSendToUsersParams{
				UserIDs:     userIDs,
				MessageType: websocket.MessageText,
				Data:        data,
			},
		)

		if err != nil {
			b.Fatal(err)
		}
	}
}
