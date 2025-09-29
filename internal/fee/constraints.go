package fee

const PLUGIN_TYPE = "fee"

// Task Definitions
const TypeFeeLoad = "fees:load"            // Load list of pending fees into the db from the verifier
const TypeFeeTransact = "fees:transaction" // Collect a list of loaded fees from the users wallet
const TypeFeePostTx = "fees:post_tx"       // Check the status of the fee runs
