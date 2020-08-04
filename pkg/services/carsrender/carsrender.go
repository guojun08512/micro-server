package carsrender

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"

	fsdk "git.keyayun.com/bohaoc/seal-file-sdk"
	"github.com/gorilla/websocket"
	cbytes "github.com/micro/go-micro/v2/codec/bytes"
	"keyayun.com/seal-micro-runner/pkg/config"
	"keyayun.com/seal-micro-runner/pkg/logger"
	pb "keyayun.com/seal-micro-runner/pkg/proto"
	"keyayun.com/seal-micro-runner/pkg/redis"
	"keyayun.com/seal-micro-runner/pkg/sealclient"
	"keyayun.com/seal-micro-runner/pkg/services"
)

type prepareParams struct {
	port       int
	workItemID string
	baseURI    string

	ready chan struct{}
	ctx   context.Context
	ws    *websocket.Conn
}

type carsRenderService struct {
	manifest *pb.ManifestInfo
	client   *sealclient.SealClient

	workers     map[string]*prepareParams
	workerPreCh chan *prepareParams
	mu          sync.Mutex

	serverID string
	rootPath string
}

const timeout = 10

var (
	log  = logger.WithNamespace("cars.render")
	conf = config.Config
	pl   = new(portPool)
)

type portPool struct {
	items []int
	lock  sync.RWMutex
}

func (pl *portPool) pull() (int, error) {
	nLen := len(pl.items)
	if nLen == 0 {
		return 0, errors.New("PortPool Is Empty")
	}
	pl.lock.Lock()
	item := pl.items[nLen-1]
	pl.items = pl.items[:len(pl.items)-1]
	pl.lock.Unlock()
	return item, nil
}

func (pl *portPool) push(port int) {
	pl.lock.Lock()
	pl.items = append(pl.items, port)
	pl.lock.Unlock()
}

func (pl *portPool) set(port int) {
	pl.items = append(pl.items, port)
}

func init() {
	var ports = conf.GetIntSlice("task.cars.render.ports")
	for _, port := range ports {
		pl.set(port)
	}
}

func NewCarsRenderService() *carsRenderService {
	s := &carsRenderService{
		manifest: &pb.ManifestInfo{
			Name:        "carsRender",
			Description: "provide cars render services",
			Version:     services.DefaultVersion,
			Categories:  []string{},
			Repository:  services.ServiceRepository,
			Scope: []string{
				sealclient.WorkItems,
				sealclient.SealFiles,
				sealclient.CarsProjectDoc,
			},
			Params: []*pb.Param{
				{
					Name:        "workItemID",
					Type:        "string",
					Description: "keyayun.seal.workItem DocID",
				},
			},
		},
		workers:     make(map[string]*prepareParams),
		workerPreCh: make(chan *prepareParams),
		rootPath:    "/mnt",
	}
	return s
}

func (c *carsRenderService) InitService(serverID string) {
	c.serverID = serverID
	go func() {
		for param := range c.workerPreCh {
			fmt.Println(param.workItemID)
			close(param.ready)
		}
	}()

	go func() {
		for {
			time.Sleep(time.Second * 6)
			var ids []string
			for id, v := range c.workers {
				if v.port > 0 && v.ws == nil {
					ids = append(ids, id)
				}
			}
			for _, id := range ids {
				c.delPreParams(id)
			}
		}
	}()
}

func (c *carsRenderService) putPreParams(id string, req *prepareParams) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workers[id] = req
}

func (c *carsRenderService) getPreParams(id string) (*prepareParams, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if r, ok := c.workers[id]; ok {
		return r, nil
	}
	return nil, os.ErrNotExist
}

func (c *carsRenderService) delPreParams(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	param := c.workers[id]
	if param == nil {
		return os.ErrNotExist
	}
	pl.push(c.workers[id].port)
	if param.ws != nil {
		param.ws.Close()
		param.ws = nil
	}
	delete(c.workers, id)
	return nil
}

func (c *carsRenderService) setClient(token *pb.TokenModel) {
	c.client = &sealclient.SealClient{
		SealClient: fsdk.SealClient{
			Authorizer: &fsdk.BearerAuthorizer{Token: token.AccessToken},
			Scheme:     token.Scheme,
			Domain:     token.Domain,
			HTTPClient: &http.Client{Timeout: time.Second * sealclient.HttpClientTimeOut},
		},
		Token: token,
	}
}

func (c *carsRenderService) Manifest(_ context.Context, _ *pb.ManifestRequest, rsp *pb.ManifestInfo) error {
	if c.manifest == nil {
		return errors.New("manifest is not Init")
	}
	rsp.Name = c.manifest.Name
	rsp.Description = c.manifest.Description
	rsp.Version = c.manifest.Version
	rsp.Categories = c.manifest.Categories
	rsp.Repository = c.manifest.Repository
	rsp.Scope = c.manifest.Scope
	rsp.Params = c.manifest.Params
	rsp.Services = c.manifest.Services
	return nil
}

func (c *carsRenderService) Register(ctx context.Context, req *pb.TokenModel, rsp *pb.TokenResponse) error {
	defer log.Infof("%s register success, token = %s", c.manifest.Name, req.AccessToken)
	serviceName := c.manifest.Name + "_" + req.Domain
	err := redis.RegisterInstance(serviceName, req)
	if err != nil {
		log.Errorf("carsRenderService Register failed when RegisterInstance: %s", err)
		return err
	}
	return nil
}

func (c *carsRenderService) Update(ctx context.Context, req *pb.TokenModel, rsp *pb.TokenResponse) error {
	defer log.Infof("%s register success, token = %s", c.manifest.Name, req.AccessToken)
	serviceName := c.manifest.Name + "_" + req.Domain
	err := redis.RegisterInstance(serviceName, req)
	if err != nil {
		log.Errorf("carsRenderService Register failed when RegisterInstance: %s", err)
		return err
	}
	return nil
}

func (c *carsRenderService) UnRegister(ctx context.Context, req *pb.TokenModel, rsp *pb.TokenResponse) error {
	defer log.Infof("%s register success, token = %s", c.manifest.Name, req.AccessToken)
	serviceName := c.manifest.Name + "_" + req.Domain
	err := redis.UnregisterInstance(serviceName, req)
	if err != nil {
		log.Errorf("carsRenderService Register failed when RegisterInstance: %s", err)
		return err
	}
	return nil
}

func (c *carsRenderService) Start(_ context.Context, req *pb.StartRequest, rsp *pb.StartResponse) error {
	tokenID := c.manifest.Name + "_" + req.Domain
	token, err := redis.GetInstance(tokenID)
	if err != nil {
		log.Errorf("carsRenderService Start failed: %v", err)
		return err
	}
	c.setClient(token)
	ctx, cancel := context.WithCancel(context.TODO())
	param := &prepareParams{
		ready:      make(chan struct{}),
		ctx:        ctx,
		workItemID: req.WorkItemID,
		baseURI:    req.BaseWSlink,
	}
	streamUris := make([]string, 4)
	stopUris := make([]string, 4)
	ports := make([]int, 4)
	select {
	case c.workerPreCh <- param:
		for i := 0; i < 4; i++ {
			uid := uuid.NewV4().String()
			port, err := pl.pull()
			if err != nil {
				return err
			}
			streamUrl := fmt.Sprintf("%sServices.Stream?_id=%s&_sid=%s", r.baseURI, c.serverID, uid)
			stopUrl := fmt.Sprintf("%sServices.Stop?_id=%s&_sid=%s", r.baseURI, c.serverID, uid)
			param.port = port
			ports[i] = port
			streamUris[i] = streamUrl
			stopUris[i] = stopUrl
			c.putPreParams(uid, param)
		}
	case <-time.After(time.Second * timeout):
		return errors.New(fmt.Sprintf("workspace prepare failed: workItemID(%s)", param.workItemID))
	}
	select {
	case <-param.ready:
		rsp.StreamUrls = streamUris
		rsp.StopUrls = stopUris
		//fmt.Println(ready.Workspace)
		fmt.Println("go")
	case <-time.After(time.Second * timeout):
		cancel()
		return errors.New("docker start up failed")
	}
	return nil
}

func (c *carsRenderService) Stop(ctx context.Context, req *pb.StopRequest, rsp *pb.StopResponse) error {
	defer log.Infof("End.Stop")
	err := c.delPreParams(req.XSid)
	if err != nil {
		log.Errorf("carsRenderService Stop failed: %v", err)
		return err
	}
	return nil
}

func (c *carsRenderService) Stream(ctx context.Context, stream pb.Services_StreamStream) error {
	data, err := stream.Recv()
	if err != nil {
		log.Errorf("carsRenderService Stream.Recv failed: %v", err)
		return err
	}
	param, err := c.getPreParams(data.XSid)
	if err != nil {
		log.Errorf("carsRenderService Stream getParam: %v", err)
		return err
	}

	ws, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s:%d/ws", conf.GetString("host"), param.port), nil)
	if err != nil {
		log.Errorf("carsRenderService Stream dial failed: %v", err)
		return err
	}
	param.ws = ws
	defer ws.Close()
	go func() {
		defer stream.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stream.Context().Done():
				return
			default:
			}
			_, message, err := ws.ReadMessage()
			if err != nil {
				log.Errorf("carsRenderService Stream ws recv failed: %v", err)
				return
			}
			log.Printf("Stream ws recv: %s", string(message))
			err = stream.SendMsg(&cbytes.Frame{Data: message})
			if err != nil {
				log.Errorf("carsRenderService stream write failed: %v", err)
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-stream.Context().Done():
			return nil
		default:
		}
		var data cbytes.Frame
		err := stream.RecvMsg(&data)
		if err != nil {
			log.Errorf("carsRenderService stream recv failed: %v", err)
			return err
		}
		log.Infof("carsRenderService stream.recv (%s)", string(data.Data))
		err = ws.WriteMessage(websocket.TextMessage, data.Data)
		if err != nil {
			log.Errorf("carsRenderService ws write failed: %v", err)
			return err
		}
	}
	return nil
}
