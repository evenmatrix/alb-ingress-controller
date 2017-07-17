package awsutil

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/coreos/alb-ingress-controller/log"
	"github.com/karlseguin/ccache"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prometheus.MustRegister(OnUpdateCount)
	prometheus.MustRegister(ReloadCount)
	prometheus.MustRegister(AWSErrorCount)
	prometheus.MustRegister(ManagedIngresses)
	prometheus.MustRegister(AWSCache)
	prometheus.MustRegister(AWSRequest)
}

type APICache struct {
	cache *ccache.Cache
}

var (
	// Session is a pointer to the AWS session
	Session *session.Session
	// Route53svc is a pointer to the awsutil Route53 service
	Route53svc *Route53
	// ALBsvc is a pointer to the awsutil ELBV2 service
	ALBsvc *ELBV2
	// Ec2svc is a pointer to the awsutil EC2 service
	Ec2svc *EC2
	// ACMsvc is a pointer to the awsutil ACM service
	ACMsvc *ACM
	// IAMsvc is a pointer to the awsutil IAM service
	IAMsvc *IAM
	// AWSDebug turns on AWS API debug logging
	AWSDebug bool

	// OnUpdateCount is a counter of the controller OnUpdate calls
	OnUpdateCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "albingress_updates",
		Help: "Number of times OnUpdate has been called.",
	},
	)

	// ReloadCount is a counter of the controller Reload calls
	ReloadCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "albingress_reloads",
		Help: "Number of times Reload has been called.",
	},
	)

	// AWSErrorCount is a counter of AWS errors
	AWSErrorCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "albingress_aws_errors",
		Help: "Number of errors from the AWS API",
	},
		[]string{"service", "request"},
	)

	// ManagedIngresses contains the current tally of managed ingresses
	ManagedIngresses = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "albingress_managed_ingresses",
		Help: "Number of ingresses being managed",
	})

	// AWSCache contains the hits and misses to our caches
	AWSCache = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "albingress_cache",
		Help: "Number of ingresses being managed",
	},
		[]string{"cache", "action"})

	// AWSRequest contains the requests made to the AWS API
	AWSRequest = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "albingress_aws_requests",
		Help: "Number of requests made to the AWS API",
	},
		[]string{"service", "operation"})
)

// NewSession returns an AWS session based off of the provided AWS config
func NewSession(awsconfig *aws.Config) *session.Session {
	session, err := session.NewSession(awsconfig)
	if err != nil {
		AWSErrorCount.With(prometheus.Labels{"service": "AWS", "request": "NewSession"}).Add(float64(1))
		log.Errorf("Failed to create AWS session. Error: %s.", "aws", err.Error())
		return nil
	}
	session.Handlers.Send.PushFront(func(r *request.Request) {
		AWSRequest.With(prometheus.Labels{"service": r.ClientInfo.ServiceName, "operation": r.Operation.Name}).Add(float64(1))
		if AWSDebug {
			log.Infof("Request: %s/%s, Payload: %s", "aws", r.ClientInfo.ServiceName, r.Operation, r.Params)
		}
	})
	return session
}

// Prettify wraps github.com/aws/aws-sdk-go/aws/awsutil.Prettify. Preventing the need to import it
// in each package.
func Prettify(i interface{}) string {
	return awsutil.Prettify(i)
}

// DeepEqual wraps github.com/aws/aws-sdk-go/aws/awsutil.Prettify. Preventing the need to import it
// in each package.
func DeepEqual(a interface{}, b interface{}) bool {
	return awsutil.DeepEqual(a, b)
}

// Get retrieves a key in the API cache. If they key doesn't exist or it expired, nil is returned.
func (ac APICache) Get(key string) *ccache.Item {
	i := ac.cache.Get(key)
	if i == nil || i.Expired() {
		return nil
	}
	return i
}

// Set add a key and value to the API cache.
func (ac APICache) Set(key string, value interface{}, duration time.Duration) {
	ac.cache.Set(key, value, duration)
}
