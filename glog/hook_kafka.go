package glog

import (
	"errors"
	"github.com/Shopify/sarama"
	"log"
	"net"
	"reflect"
	"strings"
	"time"
)

const (
	keyKafkaEnabled = "logger.kafka.enabled"
	keyKafkaLevel   = "logger.kafka.level"
	keyKafkaTopic   = "logger.kafka.topic"
	keyKafkaBrokers = "logger.kafka.brokers"
	keyKafkaApp     = "logger.kafka.app"
	CloseReopen     = 0
	CloseExit       = 1
)

type kafkaHook struct {
	BaseHook
	producer *mqProducer
	topic    string
}

func (hook *kafkaHook) Setup() error {
	hook.baseSetup()

	broker := v.GetString(keyKafkaBrokers)
	if broker == "" {
		return errors.New("mq.kafka broker empty")
	}
	topic := v.GetString(keyKafkaTopic)
	hook.producer = NewMqProducer(topic+"-"+keyKafkaApp, broker)
	if err := hook.producer.Start(); err != nil {
		return err
	}
	hook.writer = &kwriter{producer: hook.producer, topic: topic}
	return nil
}

type kwriter struct {
	producer *mqProducer
	topic    string
}

func (w *kwriter) Write(p []byte) (int, error) {
	err := w.producer.SendMessage(w.topic, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

var _InitKafkaHook = func() interface{} {
	cli.Bool(keyKafkaEnabled, false, "logger.kafka.enabled")
	cli.String(keyKafkaLevel, "", "logger.kafka.level") // DONOT set default level in pflag

	registerHook("kafka", reflect.TypeOf(FileHook{}))

	return nil
}()

type mqProducer struct {
	brokerAddr []string
	resolver   *SrvResolver
	client     string
	producer   sarama.AsyncProducer
	closeCh    chan int
}

func NewMqProducer(broker string, client string) *mqProducer {
	producer := &mqProducer{
		client:   client,
		resolver: NewSrvResolver(broker),
	}
	return producer
}

func (p *mqProducer) newConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Net.MaxOpenRequests = 3
	config.Producer.RequiredAcks = sarama.WaitForLocal
	config.Producer.Idempotent = false
	config.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	config.Producer.Return.Successes = false
	config.Producer.Return.Errors = true
	config.Metadata.RefreshFrequency = time.Second * 5
	config.Version = sarama.V2_0_0_0
	if p.client != "" {
		config.ClientID = p.client
	}
	return config
}

func (p *mqProducer) createProducer() error {

	producer, err := sarama.NewAsyncProducer(p.brokerAddr, p.newConfig())
	if err != nil {
		log.Println(err)
		return err
	}
	p.producer = producer
	p.closeCh = make(chan int, 0)
	return nil
}

func (p *mqProducer) Start() error {

	p.resolver.Start()
	p.brokerAddr = p.resolver.GetIPAddrs()
	if err := p.createProducer(); err != nil {
		log.Println("create produer error:", err)
		return err
	}
	go func() {
		for {
			if p.processSending() == CloseExit {
				return
			}
			for err := p.createProducer(); err != nil; {
				log.Println("create produer error:", err)
				time.Sleep(time.Second)
				continue
			}
		}
	}()

	return nil
}

func (p *mqProducer) Stop() {
	//send close cmd
	p.closeCh <- CloseExit
	//wait the cmd to be processed
	<-p.closeCh

	p.producer.Close()
}

func (p *mqProducer) SendMessage(topic string, message []byte) error {
	msg := &sarama.ProducerMessage{}
	msg.Value = sarama.ByteEncoder(message)
	msg.Topic = topic
	p.producer.Input() <- msg

	return nil
}

func (p *mqProducer) processSending() int {
	defer func() {
		if p.producer != nil {
			close(p.closeCh)
			p.producer.Close()
		}
	}()
	for {
		select {
		case ips := <-p.resolver.IPchan():
			log.Println(ips)
			p.brokerAddr = ips
			//p.restart()
		case err := <-p.producer.Errors():
			if err != nil && err.Err != nil {
				p.processError(err.Err, err.Msg)
			}
		case code := <-p.closeCh:
			log.Println("recv close cmd, code=", code)
			return code
		}
	}
}

func (p *mqProducer) restart() {
	if p.closeCh == nil {
		return
	}
	go func() {
		p.closeCh <- CloseReopen
	}()
}

func (p *mqProducer) processError(err error, msg *sarama.ProducerMessage) {
	log.Println("err:", err)
	switch err.(type) {
	case sarama.KError:
		var kerr = err.(sarama.KError)
		if kerr == sarama.ErrOutOfOrderSequenceNumber {
			log.Println("need to shutdown the producer and create an new one")
			//need to shutdown the producer and create an new one
			p.restart()
		}
	}
}

//主要用在k8s的有状态服务(headless)的服务发现中，这些服务没有cluster IP，通过域名可以直接解析到
//service 对应pod的ip(域名),在客户端做负载均衡的组件上需要这样的功能
type SrvResolver struct {
	service string
	port    string
	ipaddr  []string
	ch      chan []string
	t       *time.Ticker
	done    chan int
}

func NewSrvResolver(service string) *SrvResolver {
	p := strings.Split(service, ":")
	return &SrvResolver{
		service: p[0],
		port:    p[1],
		ipaddr:  make([]string, 0),
		done:    make(chan int, 0),
		ch:      make(chan []string, 1),
	}
}

func (s *SrvResolver) Start() {
	s.background()
	s.t = time.NewTicker(time.Duration(time.Second * 5))
	go func() {
		for {
			select {
			case <-s.t.C:
				s.background()
			case <-s.done:
				close(s.done)
				close(s.ch)
				return
			}
		}
	}()
}

func (s *SrvResolver) Stop() {
	if s.t != nil {
		s.t.Stop()
	}
	s.done <- 1
}

func (s *SrvResolver) GetIPAddrs() []string {
	ipports := <-s.ch
	return ipports
}

func (s *SrvResolver) IPchan() chan []string {
	return s.ch
}

func (s *SrvResolver) resolve() []string {
	if s.service == "" {
		return nil
	}
	_, addrs, err := net.LookupSRV("", "", s.service)
	if err != nil {
		return nil
	}
	cname := make([]string, 0)
	for _, a := range addrs {
		cname = append(cname, a.Target)
	}
	return cname
}

func (s *SrvResolver) background() {
	ips := s.resolve()
	if len(ips) == 0 {
		return
	}

	change := false
	if len(ips) != len(s.ipaddr) {
		s.ipaddr = make([]string, 0, len(ips))
		s.ipaddr = append(s.ipaddr, ips...)
		change = true
	} else {
		count := len(ips)
		for _, ipn := range ips {
			for _, ipo := range s.ipaddr {
				if ipn == ipo {
					count--
					break
				}
			}
		}
		if count > 0 {
			s.ipaddr = make([]string, 0, len(ips))
			s.ipaddr = append(s.ipaddr, ips...)
			change = true
		}
	}
	if change {
		ipports := make([]string, 0, len(s.ipaddr))
		for _, ip := range s.ipaddr {
			ipports = append(ipports, ip+":"+s.port)
		}
		log.Println("New IP:", ipports)
		s.ch <- ipports
	}
}
