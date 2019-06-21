// Package broker holds the code that users of the skeleton write for their
// broker. To make a broker, fill out:
//
// - The Options type, which holds options for the broker
// - The AddFlags function, which adds CLI flags for an Options
// - The methods of the BusinessLogic type, which implements the broker's
//   business logic
// - The NewBusinessLogic function, which creates a BusinessLogic from the
//   Options the program is run with
package broker

// OSBVersion is the Open Service Broker Spec version for this broker
const OSBVersion = "2.13"
