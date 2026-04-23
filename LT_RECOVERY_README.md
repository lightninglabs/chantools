# Lightning Terminal Recovery Configuration

This document explains how to use the Lightning Terminal recovery configuration system to recover funds from Lightning Terminal taproot channels.

## Configuration File

The `lt_recovery_config.json` file contains all the Lightning Terminal-specific parameters that were previously hardcoded. This allows the recovery tool to be used for different Lightning Terminal recovery scenarios.

### Configuration Structure

```json
{
  "description": "Lightning Terminal Recovery Configuration",
  "version": "1.0",
  "lightning_terminal": {
    "keys": {
      "actual_internal_key": "YOUR_LIGHTNING_TERMINAL_INTERNAL_KEY",
      "remote_revocation_base": "REMOTE_PEER_REVOCATION_BASE_KEY",
      "remote_funding_key": "REMOTE_PEER_FUNDING_KEY"
    },
    "channel": {
      "type": 3630,
      "csv_delays": [144, 1008, 2016],
      "key_index": 4,
      "balance": 97751
    },
    "tapscript": {
      "actual_root": "YOUR_CHANNEL_TAPSCRIPT_ROOT"
    }
  }
}
```

### Key Parameters to Update

#### 1. Lightning Terminal Keys
- **`actual_internal_key`**: The internal key used by Lightning Terminal for your channel
- **`remote_revocation_base`**: The remote peer's revocation base point
- **`remote_funding_key`**: The remote peer's funding key from channel backup

#### 2. Channel-Specific Values
- **`type`**: Your specific channel type (e.g., 3630)
- **`csv_delays`**: CSV delay values to test (start with your channel's actual CSV delay)
- **`key_index`**: The key derivation index for your channel (usually 4)
- **`balance`**: Your channel balance (used for asset commitment scenarios)

#### 3. Tapscript Information
- **`actual_root`**: The tapscript root from your commitment transaction

## How to Find Your Values

### 1. Internal Key
Look for error messages in Lightning Terminal logs containing `internal_key=`. Example:
```
internal_key=034078498a1e314de9798be9954561727dbd3726fab244f67dcb7230d40f8a44fc
```

### 2. Remote Keys
Extract from your channel backup file (`channel.backup`):
- **Remote funding key**: `RemoteChanCfg.MultiSigKey`
- **Remote revocation base**: From channel database or logs

### 3. Channel Type
Check your channel database or Lightning Terminal logs for the exact channel type.

### 4. Tapscript Root
Look for tapscript root in commitment transaction analysis or chantools output.

## Usage

### Basic Usage
```bash
chantools rescueclosed \
  --lt_config ./your_lt_recovery_config.json \
  --force_close_addr bc1p... \
  --commit_point 03xxxx...
```

### With Custom Config Path
```bash
chantools rescueclosed \
  --lt_config /path/to/custom_config.json \
  --channeldb ~/.lnd/data/graph/mainnet/channel.db \
  --fromsummary results/summary-xxxxxx.json
```

## Configuration Templates

### Template for Different Channel Types

For **SIMPLE_TAPROOT_OVERLAY** channels:
```json
{
  "lightning_terminal": {
    "channel": {
      "type": 3630,
      "csv_delays": [144],
      "key_index": 4
    }
  }
}
```

For **Standard LND taproot** channels:
```json
{
  "lightning_terminal": {
    "channel": {
      "type": 1073741824,
      "csv_delays": [144, 1008],
      "key_index": 0
    }
  }
}
```

## Testing Configuration

### Validate Configuration
```bash
# Test that config loads correctly
chantools rescueclosed --lt_config ./test_config.json --help
```

### Debug Mode
Add verbose logging to see which scenarios are being tested:
```bash
chantools rescueclosed \
  --lt_config ./your_config.json \
  --force_close_addr bc1p... \
  --commit_point 03xxxx... \
  --verbose
```

## Common Issues

### 1. Config Not Found
```
error loading LT config: failed to read config file: no such file or directory
```
**Solution**: Ensure the config file path is correct and the file exists.

### 2. Invalid Key Format
```
failed to decode actual internal key: encoding/hex: invalid byte
```
**Solution**: Check that all keys are valid hex strings without 0x prefix.

### 3. No Match Found
If no private key is found, try:
1. Verify the `actual_internal_key` is correct
2. Check the `key_index` matches your channel
3. Adjust `csv_delays` to include your channel's actual delay
4. Verify the `channel.type` is correct

## Advanced Configuration

### Testing Multiple Scenarios
You can add multiple test scenarios for different possible configurations:

```json
{
  "testing": {
    "max_htlc_index": 20,
    "max_keys_to_test": 10000,
    "channel_types": [
      3630,
      "SimpleTaprootFeatureBit | TapscriptRootBit",
      "SimpleTaprootFeatureBit | TapscriptRootBit | AnchorOutputsBit"
    ]
  }
}
```

### Asset Commitment Scenarios
For Lightning Terminal with taproot assets:

```json
{
  "auxiliary_leaves": {
    "asset_scenarios": [
      {
        "version": 0,
        "name": "empty-commitment",
        "root_sum": 0
      },
      {
        "version": 1, 
        "name": "your-asset-commitment",
        "root_sum": 97751
      }
    ]
  }
}
```

## Security Notes

- Keep your configuration files secure as they contain channel-specific information
- Never share your `actual_internal_key` or other sensitive parameters
- Use different config files for different recovery scenarios
- Back up your working configuration once you successfully recover funds