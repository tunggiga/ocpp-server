package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	ocpp16 "github.com/lorenzodonini/ocpp-go/ocpp1.6"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logger.SetLevel(logrus.DebugLevel)
	ocppj.SetLogger(logger)

	centralSystem := ocpp16.NewCentralSystem(nil, nil)

	// Set callback handlers for connect/disconnect
	centralSystem.SetNewChargePointHandler(func(chargePoint ocpp16.ChargePointConnection) {
		log.Printf("new charge point %v connected", chargePoint.ID())
	})
	centralSystem.SetChargePointDisconnectedHandler(func(chargePoint ocpp16.ChargePointConnection) {
		log.Printf("charge point %v disconnected", chargePoint.ID())
	})

	// Set handler for profile callbacks
	handler := &CentralSystemHandler{
		logger:         logger,
		transactionIds: map[string]int{},
		mutex:          &sync.Mutex{},
	}
	centralSystem.SetCoreHandler(handler)

	go func() {
		e := echo.New()

		e.Static("/", "static")

		e.HTTPErrorHandler = func(err error, c echo.Context) {
			err = c.JSON(http.StatusInternalServerError, err.Error())
			e.DefaultHTTPErrorHandler(err, c)
		}

		e.POST("/remote_start_transaction", func(c echo.Context) error {
			params := struct {
				ClientId      string `json:"clientId"`
				IdTag         string `json:"idTag"`
				TransactionId int    `json:"transactionId"`
			}{}
			if err := c.Bind(&params); err != nil {
				return err
			}

			handler.addTransactionId(params.ClientId, params.TransactionId)

			resultChan := make(chan *core.RemoteStartTransactionConfirmation)
			errChan := make(chan error)
			err := centralSystem.RemoteStartTransaction(params.ClientId, func(result *core.RemoteStartTransactionConfirmation, err error) {
				if err != nil {
					errChan <- err
				} else {
					resultChan <- result
				}
			}, params.IdTag)
			if err != nil {
				return err
			}

			select {
			case result := <-resultChan:
				return c.JSON(http.StatusOK, result)
			case err = <-errChan:
				return err
			case <-time.After(10 * time.Second):
				return errors.New("Timeout")
			}
		})

		e.POST("/remote_stop_transaction", func(c echo.Context) error {
			params := struct {
				ClientId      string `json:"clientId"`
				TransactionId int    `json:"transactionId"`
			}{}
			if err := c.Bind(&params); err != nil {
				return err
			}

			resultChan := make(chan *core.RemoteStopTransactionConfirmation)
			errChan := make(chan error)
			err := centralSystem.RemoteStopTransaction(params.ClientId, func(result *core.RemoteStopTransactionConfirmation, err error) {
				if err != nil {
					errChan <- err
				} else {
					resultChan <- result
				}
			}, params.TransactionId)
			if err != nil {
				return err
			}

			select {
			case result := <-resultChan:
				return c.JSON(http.StatusOK, result)
			case err = <-errChan:
				return err
			case <-time.After(10 * time.Second):
				return errors.New("Timeout")
			}
		})

		e.POST("/reset", func(c echo.Context) error {
			params := struct {
				ClientId  string         `json:"clientId"`
				ResetType core.ResetType `json:"resetType"`
			}{}
			if err := c.Bind(&params); err != nil {
				return err
			}

			resultChan := make(chan *core.ResetConfirmation)
			errChan := make(chan error)
			err := centralSystem.Reset(params.ClientId, func(result *core.ResetConfirmation, err error) {
				if err != nil {
					errChan <- err
				} else {
					resultChan <- result
				}
			}, params.ResetType)
			if err != nil {
				return err
			}

			select {
			case result := <-resultChan:
				return c.JSON(http.StatusOK, result)
			case err = <-errChan:
				return err
			case <-time.After(10 * time.Second):
				return errors.New("Timeout")
			}
		})

		e.Logger.Fatal(e.Start(":8777"))
	}()

	// Start central system
	listenPort := 8887
	log.Printf("starting central system")
	centralSystem.Start(listenPort, "/{ws}")
	log.Println("stopped central system")
}

func toJson(object interface{}) string {
	data, _ := json.Marshal(object)
	return string(data)
}

const defaultHeartbeatInterval = 600

type CentralSystemHandler struct {
	logger         *logrus.Logger
	transactionIds map[string]int
	mutex          *sync.Mutex
}

func (handler *CentralSystemHandler) logRequest(chargePointId string, request interface{}, confirmation interface{}) {
	fmt.Println("----------------------------------------")
	fmt.Printf("chargePointId: %s\n", chargePointId)
	fmt.Printf("request: %s\n", toJson(request))
	fmt.Printf("response: %s\n", toJson(confirmation))
	fmt.Println("----------------------------------------")
}

func (handler *CentralSystemHandler) addTransactionId(chargePointId string, transactionId int) {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	handler.transactionIds[chargePointId] = transactionId
}

func (handler *CentralSystemHandler) OnAuthorize(chargePointId string, request *core.AuthorizeRequest) (*core.AuthorizeConfirmation, error) {
	confirmation := core.NewAuthorizationConfirmation(types.NewIdTagInfo(types.AuthorizationStatusAccepted))
	handler.logRequest(chargePointId, request, confirmation)
	return confirmation, nil
}

func (handler *CentralSystemHandler) OnBootNotification(chargePointId string, request *core.BootNotificationRequest) (*core.BootNotificationConfirmation, error) {
	confirmation := core.NewBootNotificationConfirmation(types.NewDateTime(time.Now()), defaultHeartbeatInterval, core.RegistrationStatusAccepted)
	handler.logRequest(chargePointId, request, confirmation)
	return confirmation, nil
}

func (handler *CentralSystemHandler) OnDataTransfer(chargePointId string, request *core.DataTransferRequest) (*core.DataTransferConfirmation, error) {
	confirmation := core.NewDataTransferConfirmation(core.DataTransferStatusAccepted)
	handler.logRequest(chargePointId, request, confirmation)
	return confirmation, nil
}

func (handler *CentralSystemHandler) OnHeartbeat(chargePointId string, request *core.HeartbeatRequest) (*core.HeartbeatConfirmation, error) {
	confirmation := core.NewHeartbeatConfirmation(types.NewDateTime(time.Now()))
	handler.logRequest(chargePointId, request, confirmation)
	return confirmation, nil
}

func (handler *CentralSystemHandler) OnMeterValues(chargePointId string, request *core.MeterValuesRequest) (*core.MeterValuesConfirmation, error) {
	confirmation := core.NewMeterValuesConfirmation()
	handler.logRequest(chargePointId, request, confirmation)
	return confirmation, nil
}

func (handler *CentralSystemHandler) OnStatusNotification(chargePointId string, request *core.StatusNotificationRequest) (*core.StatusNotificationConfirmation, error) {
	confirmation := core.NewStatusNotificationConfirmation()
	handler.logRequest(chargePointId, request, confirmation)
	return confirmation, nil
}

func (handler *CentralSystemHandler) OnStartTransaction(chargePointId string, request *core.StartTransactionRequest) (*core.StartTransactionConfirmation, error) {
	transactionId := 1
	handler.mutex.Lock()
	if id, ok := handler.transactionIds[chargePointId]; ok {
		transactionId = id
		delete(handler.transactionIds, chargePointId)
	}
	handler.mutex.Unlock()
	confirmation := core.NewStartTransactionConfirmation(types.NewIdTagInfo(types.AuthorizationStatusAccepted), transactionId)
	handler.logRequest(chargePointId, request, confirmation)
	return confirmation, nil
}

func (handler *CentralSystemHandler) OnStopTransaction(chargePointId string, request *core.StopTransactionRequest) (*core.StopTransactionConfirmation, error) {
	confirmation := core.NewStopTransactionConfirmation()
	confirmation.IdTagInfo = types.NewIdTagInfo(types.AuthorizationStatusAccepted)
	handler.logRequest(chargePointId, request, confirmation)
	return confirmation, nil
}
