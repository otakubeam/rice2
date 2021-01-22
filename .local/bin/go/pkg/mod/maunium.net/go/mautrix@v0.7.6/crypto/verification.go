// Copyright (c) 2020 Nikos Filippakis
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build !nosas

package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/canonicaljson"
	"maunium.net/go/mautrix/crypto/olm"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// ErrUnknownTransaction is returned when a key verification message is received with an unknown transaction ID.
var ErrUnknownTransaction = errors.New("Unknown transaction")

// ErrUnknownVerificationMethod is returned when the verification method in a received m.key.verification.start is unknown.
var ErrUnknownVerificationMethod = errors.New("Unknown verification method")

type VerificationHooks interface {
	// VerifySASMatch receives the generated SAS and its method, as well as the device that is being verified.
	// It returns whether the given SAS match with the SAS displayed on other device.
	VerifySASMatch(otherDevice *DeviceIdentity, sas SASData) bool
	// VerificationMethods returns the list of supported verification methods in order of preference.
	// It must contain at least the decimal method.
	VerificationMethods() []VerificationMethod
	OnCancel(cancelledByUs bool, reason string, reasonCode event.VerificationCancelCode)
	OnSuccess()
}

type VerificationRequestResponse int

const (
	AcceptRequest VerificationRequestResponse = iota
	RejectRequest
	IgnoreRequest
)

// sendToOneDevice sends a to-device event to a single device.
func (mach *OlmMachine) sendToOneDevice(userID id.UserID, deviceID id.DeviceID, eventType event.Type, content interface{}) error {
	_, err := mach.Client.SendToDevice(eventType, &mautrix.ReqSendToDevice{
		Messages: map[id.UserID]map[id.DeviceID]*event.Content{
			userID: {
				deviceID: {
					Parsed: content,
				},
			},
		},
	})

	return err
}

func (mach *OlmMachine) getPKAndKeysMAC(sas *olm.SAS, sendingUser id.UserID, sendingDevice id.DeviceID, receivingUser id.UserID, receivingDevice id.DeviceID,
	transactionID string, signingKey id.SigningKey, mainKeyID id.KeyID, keys map[id.KeyID]string) (string, string, error) {
	sasInfo := "MATRIX_KEY_VERIFICATION_MAC" +
		sendingUser.String() + sendingDevice.String() +
		receivingUser.String() + receivingDevice.String() +
		transactionID

	keyIDString := mainKeyID.String()
	if keys != nil {
		keyIDStrings := make([]string, len(keys))
		i := 0
		for keyID := range keys {
			keyIDStrings[i] = keyID.String()
			i++
		}
		sort.Sort(sort.StringSlice(keyIDStrings))
		keyIDString = strings.Join(keyIDStrings, ",")
	}

	pubKeyMac, err := sas.CalculateMAC([]byte(signingKey), []byte(sasInfo+mainKeyID.String()))
	if err != nil {
		return "", "", err
	}
	mach.Log.Trace("sas.CalculateMAC(\"%s\", \"%s\") -> \"%s\"", signingKey, sasInfo+mainKeyID.String(), string(pubKeyMac))

	keysMac, err := sas.CalculateMAC([]byte(keyIDString), []byte(sasInfo+"KEY_IDS"))
	if err != nil {
		return "", "", err
	}
	mach.Log.Trace("sas.CalculateMAC(\"%s\", \"%s\") -> \"%s\"", keyIDString, sasInfo+"KEY_IDS", string(keysMac))

	return string(pubKeyMac), string(keysMac), nil
}

// verificationState holds all the information needed for the state of a SAS verification with another device.
type verificationState struct {
	sas                 *olm.SAS
	otherDevice         *DeviceIdentity
	initiatedByUs       bool
	verificationStarted bool
	keyReceived         bool
	sasMatched          chan bool
	commitment          string
	startEventCanonical string
	chosenSASMethod     VerificationMethod
	hooks               VerificationHooks
	extendTimeout       context.CancelFunc
	lock                sync.Mutex
}

// getTransactionState retrieves the given transaction's state, or cancels the transaction if it cannot be found or there is a mismatch.
func (mach *OlmMachine) getTransactionState(transactionID string, userID id.UserID) (*verificationState, error) {
	verStateInterface, ok := mach.keyVerificationTransactionState.Load(userID.String() + ":" + transactionID)
	if !ok {
		_ = mach.SendSASVerificationCancel(userID, id.DeviceID("*"), transactionID, "Unknown transaction: "+transactionID, event.VerificationCancelUnknownTransaction)
		return nil, ErrUnknownTransaction
	}
	verState := verStateInterface.(*verificationState)
	if verState.otherDevice.UserID != userID {
		reason := fmt.Sprintf("Unknown user for transaction %v: %v", transactionID, userID)
		_ = mach.SendSASVerificationCancel(userID, id.DeviceID("*"), transactionID, reason, event.VerificationCancelUserMismatch)
		mach.keyVerificationTransactionState.Delete(userID.String() + ":" + transactionID)
		return nil, errors.New(reason)
	}
	return verState, nil
}

// handleVerificationStart handles an incoming m.key.verification.start message.
// It initializes the state for this SAS verification process and stores it.
func (mach *OlmMachine) handleVerificationStart(userID id.UserID, content *event.VerificationStartEventContent, timeout time.Duration) {
	mach.Log.Debug("Received verification start from %v", content.FromDevice)
	otherDevice, err := mach.GetOrFetchDevice(userID, content.FromDevice)
	if err != nil {
		mach.Log.Error("Could not find device %v of user %v", content.FromDevice, userID)
		return
	}
	warnAndCancel := func(logReason, cancelReason string) {
		mach.Log.Warn("Canceling verification transaction %v as it %s", content.TransactionID, logReason)
		_ = mach.SendSASVerificationCancel(otherDevice.UserID, otherDevice.DeviceID, content.TransactionID, cancelReason, event.VerificationCancelUnknownMethod)
	}
	switch {
	case content.Method != event.VerificationMethodSAS:
		warnAndCancel("is not SAS", "Only SAS method is supported")
	case !content.SupportsKeyAgreementProtocol(event.KeyAgreementCurve25519HKDFSHA256):
		warnAndCancel("does not support key agreement protocol curve25519-hkdf-sha256",
			"Only curve25519-hkdf-sha256 key agreement protocol is supported")
	case !content.SupportsHashMethod(event.VerificationHashSHA256):
		warnAndCancel("does not support SHA256 hashing", "Only SHA256 hashing is supported")
	case !content.SupportsMACMethod(event.HKDFHMACSHA256):
		warnAndCancel("does not support MAC method hkdf-hmac-sha256", "Only hkdf-hmac-sha256 MAC method is supported")
	case !content.SupportsSASMethod(event.SASDecimal):
		warnAndCancel("does not support decimal SAS", "Decimal SAS method must be supported")
	default:
		mach.actuallyStartVerification(userID, content, otherDevice, timeout)
	}
}

func (mach *OlmMachine) actuallyStartVerification(userID id.UserID, content *event.VerificationStartEventContent, otherDevice *DeviceIdentity, timeout time.Duration) {
	resp, hooks := mach.AcceptVerificationFrom(content.TransactionID, otherDevice)
	if resp == AcceptRequest {
		sasMethods := commonSASMethods(hooks, content.ShortAuthenticationString)
		if len(sasMethods) == 0 {
			mach.Log.Error("No common SAS methods: %v", content.ShortAuthenticationString)
			_ = mach.SendSASVerificationCancel(otherDevice.UserID, otherDevice.DeviceID, content.TransactionID, "No common SAS methods", event.VerificationCancelUnknownMethod)
			return
		}
		verState := &verificationState{
			sas:                 olm.NewSAS(),
			otherDevice:         otherDevice,
			initiatedByUs:       false,
			verificationStarted: true,
			keyReceived:         false,
			sasMatched:          make(chan bool, 1),
			hooks:               hooks,
			chosenSASMethod:     sasMethods[0],
		}
		verState.lock.Lock()
		defer verState.lock.Unlock()

		_, loaded := mach.keyVerificationTransactionState.LoadOrStore(userID.String()+":"+content.TransactionID, verState)
		if loaded {
			// transaction already exists
			mach.Log.Error("Transaction %v already exists, canceling", content.TransactionID)
			_ = mach.SendSASVerificationCancel(otherDevice.UserID, otherDevice.DeviceID, content.TransactionID, "Transaction already exists", event.VerificationCancelUnexpectedMessage)
			return
		}

		mach.timeoutAfter(verState, content.TransactionID, timeout)

		err := mach.SendSASVerificationAccept(userID, content, verState.sas.GetPubkey(), sasMethods)
		if err != nil {
			mach.Log.Error("Error accepting SAS verification: %v", err)
		}
	} else if resp == RejectRequest {
		mach.Log.Debug("Not accepting SAS verification %v from %v of user %v", content.TransactionID, otherDevice.DeviceID, otherDevice.UserID)
		err := mach.SendSASVerificationCancel(otherDevice.UserID, otherDevice.DeviceID, content.TransactionID, "Not accepted by user", event.VerificationCancelByUser)
		if err != nil {
			mach.Log.Error("Error canceling SAS verification: %v", err)
		}
	} else {
		mach.Log.Debug("Ignoring SAS verification %v from %v of user %v", content.TransactionID, otherDevice.DeviceID, otherDevice.UserID)
	}
}

func (mach *OlmMachine) timeoutAfter(verState *verificationState, transactionID string, timeout time.Duration) {
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), timeout)
	verState.extendTimeout = timeoutCancel
	go func() {
		mapKey := verState.otherDevice.UserID.String() + ":" + transactionID
		for {
			<-timeoutCtx.Done()
			// when timeout context is done
			verState.lock.Lock()
			// if transaction not active anymore, return
			if _, ok := mach.keyVerificationTransactionState.Load(mapKey); !ok {
				verState.lock.Unlock()
				return
			}
			if timeoutCtx.Err() == context.DeadlineExceeded {
				// if deadline exceeded cancel due to timeout
				mach.keyVerificationTransactionState.Delete(mapKey)
				_ = mach.callbackAndCancelSASVerification(verState, transactionID, "Timed out", event.VerificationCancelByTimeout)
				mach.Log.Warn("Verification transaction %v is canceled due to timing out", transactionID)
				verState.lock.Unlock()
				return
			}
			// otherwise the cancel func was called, so the timeout is reset
			mach.Log.Debug("Extending timeout for transaction %v", transactionID)
			timeoutCtx, timeoutCancel = context.WithTimeout(context.Background(), timeout)
			verState.extendTimeout = timeoutCancel
			verState.lock.Unlock()
		}
	}()
}

// handleVerificationAccept handles an incoming m.key.verification.accept message.
// It continues the SAS verification process by sending the SAS key message to the other device.
func (mach *OlmMachine) handleVerificationAccept(userID id.UserID, content *event.VerificationAcceptEventContent) {
	mach.Log.Debug("Received verification accept for transaction %v", content.TransactionID)
	verState, err := mach.getTransactionState(content.TransactionID, userID)
	if err != nil {
		mach.Log.Error("Error getting transaction state: %v", err)
		return
	}
	verState.lock.Lock()
	defer verState.lock.Unlock()
	verState.extendTimeout()

	if !verState.initiatedByUs || verState.verificationStarted {
		// unexpected accept at this point
		mach.Log.Warn("Unexpected verification accept message for transaction %v", content.TransactionID)
		mach.keyVerificationTransactionState.Delete(userID.String() + ":" + content.TransactionID)
		_ = mach.callbackAndCancelSASVerification(verState, content.TransactionID, "Unexpected accept message", event.VerificationCancelUnexpectedMessage)
		return
	}

	sasMethods := commonSASMethods(verState.hooks, content.ShortAuthenticationString)
	if content.KeyAgreementProtocol != event.KeyAgreementCurve25519HKDFSHA256 ||
		content.Hash != event.VerificationHashSHA256 ||
		content.MessageAuthenticationCode != event.HKDFHMACSHA256 ||
		len(sasMethods) == 0 {

		mach.Log.Warn("Canceling verification transaction %v due to unknown parameter", content.TransactionID)
		mach.keyVerificationTransactionState.Delete(userID.String() + ":" + content.TransactionID)
		_ = mach.callbackAndCancelSASVerification(verState, content.TransactionID, "Verification uses unknown method", event.VerificationCancelUnknownMethod)
		return
	}

	key := verState.sas.GetPubkey()
	verState.commitment = content.Commitment
	verState.chosenSASMethod = sasMethods[0]
	verState.verificationStarted = true
	if err := mach.SendSASVerificationKey(userID, verState.otherDevice.DeviceID, content.TransactionID, string(key)); err != nil {
		mach.Log.Error("Error sending SAS key to other device: %v", err)
		return
	}
}

// handleVerificationKey handles an incoming m.key.verification.key message.
// It stores the other device's public key in order to acquire the SAS shared secret.
func (mach *OlmMachine) handleVerificationKey(userID id.UserID, content *event.VerificationKeyEventContent) {
	transactionID := content.TransactionID
	mach.Log.Debug("Got verification key for transaction %v: %v", transactionID, content.Key)
	verState, err := mach.getTransactionState(transactionID, userID)
	if err != nil {
		mach.Log.Error("Error getting transaction state: %v", err)
		return
	}
	verState.lock.Lock()
	defer verState.lock.Unlock()
	verState.extendTimeout()

	device := verState.otherDevice

	if !verState.verificationStarted || verState.keyReceived {
		// unexpected key at this point
		mach.Log.Warn("Unexpected verification key message for transaction %v", content.TransactionID)
		mach.keyVerificationTransactionState.Delete(userID.String() + ":" + content.TransactionID)
		_ = mach.callbackAndCancelSASVerification(verState, content.TransactionID, "Unexpected key message", event.VerificationCancelUnexpectedMessage)
		return
	}

	if err := verState.sas.SetTheirKey([]byte(content.Key)); err != nil {
		mach.Log.Error("Error setting other device's key: %v", err)
		return
	}

	verState.keyReceived = true

	if verState.initiatedByUs {
		// verify commitment string from accept message now
		expectedCommitment := olm.NewUtility().Sha256(content.Key + verState.startEventCanonical)
		mach.Log.Debug("Received commitment: %v Expected: %v", verState.commitment, expectedCommitment)
		if expectedCommitment != verState.commitment {
			mach.Log.Warn("Canceling verification transaction %v due to commitment mismatch", transactionID)
			mach.keyVerificationTransactionState.Delete(userID.String() + ":" + transactionID)
			_ = mach.callbackAndCancelSASVerification(verState, transactionID, "Commitment mismatch", event.VerificationCancelCommitmentMismatch)
			return
		}
	} else {
		// if verification was initiated by other device, send out our key now
		key := verState.sas.GetPubkey()
		if err := mach.SendSASVerificationKey(userID, device.DeviceID, transactionID, string(key)); err != nil {
			mach.Log.Error("Error sending SAS key to other device: %v", err)
			return
		}
	}

	// compare the SAS keys in a new goroutine and, when the verification is complete, send out the MAC
	var initUserID, acceptUserID id.UserID
	var initDeviceID, acceptDeviceID id.DeviceID
	var initKey, acceptKey string
	if verState.initiatedByUs {
		initUserID = mach.Client.UserID
		initDeviceID = mach.Client.DeviceID
		initKey = string(verState.sas.GetPubkey())
		acceptUserID = device.UserID
		acceptDeviceID = device.DeviceID
		acceptKey = content.Key
	} else {
		initUserID = device.UserID
		initDeviceID = device.DeviceID
		initKey = content.Key
		acceptUserID = mach.Client.UserID
		acceptDeviceID = mach.Client.DeviceID
		acceptKey = string(verState.sas.GetPubkey())
	}
	// use the prefered SAS method to generate a SAS
	sasMethod := verState.chosenSASMethod
	sas, err := sasMethod.GetVerificationSAS(initUserID, initDeviceID, initKey, acceptUserID, acceptDeviceID, acceptKey, transactionID, verState.sas)
	if err != nil {
		mach.Log.Error("Error generating SAS (method %v): %v", sasMethod.Type(), err)
		return
	}
	mach.Log.Debug("Generated SAS (%v): %v", sasMethod.Type(), sas)
	go func() {
		result := verState.hooks.VerifySASMatch(device, sas)
		mach.sasCompared(result, transactionID, verState)
	}()
}

// sasCompared is called asynchronously. It waits for the SAS to be compared for the verification to proceed.
// If the SAS match, then our MAC is sent out. Otherwise the transaction is canceled.
func (mach *OlmMachine) sasCompared(didMatch bool, transactionID string, verState *verificationState) {
	verState.lock.Lock()
	defer verState.lock.Unlock()
	verState.extendTimeout()
	if didMatch {
		verState.sasMatched <- true
		if err := mach.SendSASVerificationMAC(verState.otherDevice.UserID, verState.otherDevice.DeviceID, transactionID, verState.sas); err != nil {
			mach.Log.Error("Error sending verification MAC to other device: %v", err)
		}
	} else {
		verState.sasMatched <- false
	}
}

// handleVerificationMAC handles an incoming m.key.verification.mac message.
// It verifies the other device's MAC and if the MAC is valid it marks the device as trusted.
func (mach *OlmMachine) handleVerificationMAC(userID id.UserID, content *event.VerificationMacEventContent) {
	mach.Log.Debug("Got MAC for verification %v: %v, MAC for keys: %v", content.TransactionID, content.Mac, content.Keys)
	verState, err := mach.getTransactionState(content.TransactionID, userID)
	if err != nil {
		mach.Log.Error("Error getting transaction state: %v", err)
		return
	}
	verState.lock.Lock()
	defer verState.lock.Unlock()
	verState.extendTimeout()

	device := verState.otherDevice

	// we are done with this SAS verification in all cases so we forget about it
	mach.keyVerificationTransactionState.Delete(userID.String() + ":" + content.TransactionID)

	if !verState.verificationStarted || !verState.keyReceived {
		// unexpected MAC at this point
		mach.Log.Warn("Unexpected MAC message for transaction %v", content.TransactionID)
		_ = mach.callbackAndCancelSASVerification(verState, content.TransactionID, "Unexpected MAC message", event.VerificationCancelUnexpectedMessage)
		return
	}

	// do this in another goroutine as the match result might take a long time to arrive
	go func() {
		matched := <-verState.sasMatched
		verState.lock.Lock()
		defer verState.lock.Unlock()

		if !matched {
			mach.Log.Warn("SAS do not match! Canceling transaction %v", content.TransactionID)
			_ = mach.callbackAndCancelSASVerification(verState, content.TransactionID, "SAS do not match", event.VerificationCancelSASMismatch)
			return
		}

		keyID := id.NewKeyID(id.KeyAlgorithmEd25519, device.DeviceID.String())

		expectedPKMAC, expectedKeysMAC, err := mach.getPKAndKeysMAC(verState.sas, device.UserID, device.DeviceID,
			mach.Client.UserID, mach.Client.DeviceID, content.TransactionID, device.SigningKey, keyID, content.Mac)
		if err != nil {
			mach.Log.Error("Error generating MAC to match with received MAC: %v", err)
			return
		}

		mach.Log.Debug("Expected %s keys MAC, got %s", expectedKeysMAC, content.Keys)
		if content.Keys != expectedKeysMAC {
			mach.Log.Warn("Canceling verification transaction %v due to mismatched keys MAC", content.TransactionID)
			_ = mach.callbackAndCancelSASVerification(verState, content.TransactionID, "Mismatched keys MACs", event.VerificationCancelKeyMismatch)
			return
		}

		mach.Log.Debug("Expected %s PK MAC, got %s", expectedPKMAC, content.Mac[keyID])
		if content.Mac[keyID] != expectedPKMAC {
			mach.Log.Warn("Canceling verification transaction %v due to mismatched PK MAC", content.TransactionID)
			_ = mach.callbackAndCancelSASVerification(verState, content.TransactionID, "Mismatched PK MACs", event.VerificationCancelKeyMismatch)
			return
		}

		// we can finally trust this device
		device.Trust = TrustStateVerified
		err = mach.CryptoStore.PutDevice(device.UserID, device)
		if err != nil {
			mach.Log.Warn("Failed to put device after verifying: %v", err)
		}

		mach.Log.Debug("Device %v of user %v verified successfully!", device.DeviceID, device.UserID)

		verState.hooks.OnSuccess()
	}()
}

// handleVerificationCancel handles an incoming m.key.verification.cancel message.
// It cancels the verification process for the given reason.
func (mach *OlmMachine) handleVerificationCancel(userID id.UserID, content *event.VerificationCancelEventContent) {
	// make sure to not reply with a cancel to not cause a loop of cancel messages
	// this verification will get canceled even if the senders do not match
	verStateInterface, ok := mach.keyVerificationTransactionState.Load(userID.String() + ":" + content.TransactionID)
	if ok {
		go verStateInterface.(*verificationState).hooks.OnCancel(false, content.Reason, content.Code)
	}

	mach.keyVerificationTransactionState.Delete(userID.String() + ":" + content.TransactionID)
	mach.Log.Warn("SAS verification %v was canceled by %v with reason: %v (%v)",
		content.TransactionID, userID, content.Reason, content.Code)
}

// handleVerificationRequest handles an incoming m.key.verification.request message.
func (mach *OlmMachine) handleVerificationRequest(userID id.UserID, content *event.VerificationRequestEventContent) {
	mach.Log.Debug("Received verification request from %v", content.FromDevice)
	otherDevice, err := mach.GetOrFetchDevice(userID, content.FromDevice)
	if err != nil {
		mach.Log.Error("Could not find device %v of user %v", content.FromDevice, userID)
		return
	}
	if !content.SupportsVerificationMethod(event.VerificationMethodSAS) {
		mach.Log.Warn("Canceling verification transaction %v as SAS is not supported", content.TransactionID)
		_ = mach.SendSASVerificationCancel(otherDevice.UserID, otherDevice.DeviceID, content.TransactionID, "Only SAS method is supported", event.VerificationCancelUnknownMethod)
		return
	}
	resp, hooks := mach.AcceptVerificationFrom(content.TransactionID, otherDevice)
	if resp == AcceptRequest {
		mach.Log.Debug("Accepting SAS verification %v from %v of user %v", content.TransactionID, otherDevice.DeviceID, otherDevice.UserID)
		if _, err := mach.NewSASVerificationWith(otherDevice, hooks, content.TransactionID, mach.DefaultSASTimeout); err != nil {
			mach.Log.Error("Error accepting SAS verification request: %v", err)
		}
	} else if resp == RejectRequest {
		mach.Log.Debug("Rejecting SAS verification %v from %v of user %v", content.TransactionID, otherDevice.DeviceID, otherDevice.UserID)
		_ = mach.SendSASVerificationCancel(otherDevice.UserID, otherDevice.DeviceID, content.TransactionID, "Not accepted by user", event.VerificationCancelByUser)
	} else {
		mach.Log.Debug("Ignoring SAS verification %v from %v of user %v", content.TransactionID, otherDevice.DeviceID, otherDevice.UserID)
	}
}

// NewSimpleSASVerificationWith starts the SAS verification process with another device with a default timeout,
// a generated transaction ID and support for both emoji and decimal SAS methods.
func (mach *OlmMachine) NewSimpleSASVerificationWith(device *DeviceIdentity, hooks VerificationHooks) (string, error) {
	return mach.NewSASVerificationWith(device, hooks, "", mach.DefaultSASTimeout)
}

// NewSASVerificationWith starts the SAS verification process with another device.
// If the other device accepts the verification transaction, the methods in `hooks` will be used to verify the SAS match and to complete the transaction..
// If the transaction ID is empty, a new one is generated.
func (mach *OlmMachine) NewSASVerificationWith(device *DeviceIdentity, hooks VerificationHooks, transactionID string, timeout time.Duration) (string, error) {
	if transactionID == "" {
		transactionID = strconv.Itoa(rand.Int())
	}
	mach.Log.Debug("Starting new verification transaction %v with device %v of user %v", transactionID, device.DeviceID, device.UserID)

	verState := &verificationState{
		sas:                 olm.NewSAS(),
		otherDevice:         device,
		initiatedByUs:       true,
		verificationStarted: false,
		keyReceived:         false,
		sasMatched:          make(chan bool, 1),
		hooks:               hooks,
	}
	verState.lock.Lock()
	defer verState.lock.Unlock()

	startEvent, err := mach.SendSASVerificationStart(device.UserID, device.DeviceID, transactionID, hooks.VerificationMethods())
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(startEvent)
	if err != nil {
		return "", err
	}
	canonical, err := canonicaljson.CanonicalJSON(payload)
	if err != nil {
		return "", err
	}

	verState.startEventCanonical = string(canonical)
	_, loaded := mach.keyVerificationTransactionState.LoadOrStore(device.UserID.String()+":"+transactionID, verState)
	if loaded {
		return "", errors.New("Transaction already exists")
	}

	mach.timeoutAfter(verState, transactionID, timeout)

	return transactionID, nil
}

// CancelSASVerification is used by the user to cancel a SAS verification process with the given reason.
func (mach *OlmMachine) CancelSASVerification(userID id.UserID, transactionID, reason string) error {
	mapKey := userID.String() + ":" + transactionID
	verStateInterface, ok := mach.keyVerificationTransactionState.Load(mapKey)
	if !ok {
		return ErrUnknownTransaction
	}
	verState := verStateInterface.(*verificationState)
	verState.lock.Lock()
	defer verState.lock.Unlock()
	mach.Log.Trace("User canceled verification transaction %v with reason: %v", transactionID, reason)
	mach.keyVerificationTransactionState.Delete(mapKey)
	return mach.callbackAndCancelSASVerification(verState, transactionID, reason, event.VerificationCancelByUser)
}

// SendSASVerificationCancel is used to manually send a SAS cancel message process with the given reason and cancellation code.
func (mach *OlmMachine) SendSASVerificationCancel(userID id.UserID, deviceID id.DeviceID, transactionID string, reason string, code event.VerificationCancelCode) error {
	content := &event.VerificationCancelEventContent{
		TransactionID: transactionID,
		Reason:        reason,
		Code:          code,
	}
	return mach.sendToOneDevice(userID, deviceID, event.ToDeviceVerificationCancel, content)
}

// SendSASVerificationStart is used to manually send the SAS verification start message to another device.
func (mach *OlmMachine) SendSASVerificationStart(toUserID id.UserID, toDeviceID id.DeviceID, transactionID string, methods []VerificationMethod) (*event.VerificationStartEventContent, error) {
	sasMethods := make([]event.SASMethod, len(methods))
	for i, method := range methods {
		sasMethods[i] = method.Type()
	}
	content := &event.VerificationStartEventContent{
		FromDevice:                 mach.Client.DeviceID,
		TransactionID:              transactionID,
		Method:                     event.VerificationMethodSAS,
		KeyAgreementProtocols:      []event.KeyAgreementProtocol{event.KeyAgreementCurve25519HKDFSHA256},
		Hashes:                     []event.VerificationHashMethod{event.VerificationHashSHA256},
		MessageAuthenticationCodes: []event.MACMethod{event.HKDFHMACSHA256},
		ShortAuthenticationString:  sasMethods,
	}
	return content, mach.sendToOneDevice(toUserID, toDeviceID, event.ToDeviceVerificationStart, content)
}

// SendSASVerificationAccept is used to manually send an accept for a SAS verification process from a received m.key.verification.start event.
func (mach *OlmMachine) SendSASVerificationAccept(fromUser id.UserID, startEvent *event.VerificationStartEventContent, publicKey []byte, methods []VerificationMethod) error {
	if startEvent.Method != event.VerificationMethodSAS {
		reason := "Unknown verification method: " + string(startEvent.Method)
		if err := mach.SendSASVerificationCancel(fromUser, startEvent.FromDevice, startEvent.TransactionID, reason, event.VerificationCancelUnknownMethod); err != nil {
			return err
		}
		return ErrUnknownVerificationMethod
	}
	payload, err := json.Marshal(startEvent)
	if err != nil {
		return err
	}
	canonical, err := canonicaljson.CanonicalJSON(payload)
	if err != nil {
		return err
	}
	hash := olm.NewUtility().Sha256(string(publicKey) + string(canonical))
	sasMethods := make([]event.SASMethod, len(methods))
	for i, method := range methods {
		sasMethods[i] = method.Type()
	}
	content := &event.VerificationAcceptEventContent{
		TransactionID:             startEvent.TransactionID,
		Method:                    event.VerificationMethodSAS,
		KeyAgreementProtocol:      event.KeyAgreementCurve25519HKDFSHA256,
		Hash:                      event.VerificationHashSHA256,
		MessageAuthenticationCode: event.HKDFHMACSHA256,
		ShortAuthenticationString: sasMethods,
		Commitment:                hash,
	}
	return mach.sendToOneDevice(fromUser, startEvent.FromDevice, event.ToDeviceVerificationAccept, content)
}

func (mach *OlmMachine) callbackAndCancelSASVerification(verState *verificationState, transactionID, reason string, code event.VerificationCancelCode) error {
	go verState.hooks.OnCancel(true, reason, code)
	return mach.SendSASVerificationCancel(verState.otherDevice.UserID, verState.otherDevice.DeviceID, transactionID, reason, code)
}

// SendSASVerificationKey sends the ephemeral public key for a device to the partner device.
func (mach *OlmMachine) SendSASVerificationKey(userID id.UserID, deviceID id.DeviceID, transactionID string, key string) error {
	content := &event.VerificationKeyEventContent{
		TransactionID: transactionID,
		Key:           key,
	}
	return mach.sendToOneDevice(userID, deviceID, event.ToDeviceVerificationKey, content)
}

// SendSASVerificationMAC is use the MAC of a device's key to the partner device.
func (mach *OlmMachine) SendSASVerificationMAC(userID id.UserID, deviceID id.DeviceID, transactionID string, sas *olm.SAS) error {
	keyID := id.NewKeyID(id.KeyAlgorithmEd25519, mach.Client.DeviceID.String())

	signingKey := mach.account.SigningKey()
	pubKeyMac, keysMac, err := mach.getPKAndKeysMAC(sas, mach.Client.UserID, mach.Client.DeviceID, userID, deviceID, transactionID, signingKey, keyID, nil)
	if err != nil {
		return err
	}
	mach.Log.Debug("MAC of key %s is: %s", signingKey, pubKeyMac)
	mach.Log.Debug("MAC of key ID(s) %s is: %s", keyID, keysMac)

	content := &event.VerificationMacEventContent{
		TransactionID: transactionID,
		Keys:          keysMac,
		Mac: map[id.KeyID]string{
			keyID: pubKeyMac,
		},
	}
	return mach.sendToOneDevice(userID, deviceID, event.ToDeviceVerificationMAC, content)
}

func commonSASMethods(hooks VerificationHooks, otherDeviceMethods []event.SASMethod) []VerificationMethod {
	methods := make([]VerificationMethod, 0)
	for _, hookMethod := range hooks.VerificationMethods() {
		for _, otherMethod := range otherDeviceMethods {
			if hookMethod.Type() == otherMethod {
				methods = append(methods, hookMethod)
				break
			}
		}
	}
	return methods
}
