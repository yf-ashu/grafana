// Code generated by mockery v2.10.0. DO NOT EDIT.

package schedule

import (
	definitions "github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
	mock "github.com/stretchr/testify/mock"

	models "github.com/grafana/grafana/pkg/services/ngalert/models"
)

// FakeAlertsSender is an autogenerated mock type for the AlertsSender type
type FakeAlertsSender struct {
	mock.Mock
}

type FakeAlertsSender_Expecter struct {
	mock *mock.Mock
}

func (_m *FakeAlertsSender) EXPECT() *FakeAlertsSender_Expecter {
	return &FakeAlertsSender_Expecter{mock: &_m.Mock}
}

// Send provides a mock function with given fields: key, alerts
func (_m *FakeAlertsSender) Send(key models.AlertRuleKey, alerts definitions.PostableAlerts) error {
	ret := _m.Called(key, alerts)

	var r0 error
	if rf, ok := ret.Get(0).(func(models.AlertRuleKey, definitions.PostableAlerts) error); ok {
		r0 = rf(key, alerts)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// FakeAlertsSender_Send_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Send'
type FakeAlertsSender_Send_Call struct {
	*mock.Call
}

// Send is a helper method to define mock.On call
//  - key models.AlertRuleKey
//  - alerts definitions.PostableAlerts
func (_e *FakeAlertsSender_Expecter) Send(key interface{}, alerts interface{}) *FakeAlertsSender_Send_Call {
	return &FakeAlertsSender_Send_Call{Call: _e.mock.On("Send", key, alerts)}
}

func (_c *FakeAlertsSender_Send_Call) Run(run func(key models.AlertRuleKey, alerts definitions.PostableAlerts)) *FakeAlertsSender_Send_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(models.AlertRuleKey), args[1].(definitions.PostableAlerts))
	})
	return _c
}

func (_c *FakeAlertsSender_Send_Call) Return(_a0 error) *FakeAlertsSender_Send_Call {
	_c.Call.Return(_a0)
	return _c
}