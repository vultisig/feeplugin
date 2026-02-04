# Fee Collection

## Summary
Automatically collects accumulated platform fees from user vaults and transfers them to the Vultisig treasury.

## Capabilities
- **Automatic fee collection**: Collects accumulated USDC fees from user vaults on a scheduled basis
- **Debt aggregation**: Aggregates multiple fee entries (debits and credits) into a single collection transaction
- **Treasury transfers**: Sends collected fees to the designated Vultisig treasury address

## Supported Chains
Ethereum

## Parameters
| Parameter | Required | Description |
|-----------|----------|-------------|
| asset | No | Token address (defaults to USDC) |
| from_address | Yes | User's vault address |
| amount | Yes | Fee amount in smallest unit (wei) |
| to_address | Yes | Treasury address (magic constant) |
| memo | No | Optional transaction memo |

## Example User Requests
- This plugin operates automatically and does not respond to user requests
- Fee collection is triggered by the system based on accumulated fees

## Limitations
- Only supports Ethereum chain for fee collection
- Collects fees only when debt is positive (more debits than credits)
- Requires vault to have sufficient USDC balance for the fee amount
- Treasury address is configured by the system administrator
