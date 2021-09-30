With some channels `chantools forceclose` command may give you the following error:
```
[INF] CHAN: Published TX a445b928e0603d0af0b1c23876adb975c442009aa7c73c1c08bbb9a09bf1918e, response: sendrawtransaction RPC error: {"code":-26,"message":"non-mandatory-script-verify-flag (Signature must be zero for failed CHECK(MULTI)SIG operation)"}
```
Your funds may still be recovered following these steps:
1.	First of all we need to build the master branch of PSBT-Toolkit (available [here](https://github.com/benthecarman/PSBT-Toolkit)). Don’t use the last release because it has a bug that will make us fail in funds recovery.
2.	If you search for the transaction ID in your `./results/forceclose-yyyy-mm-dd.json` file (in this example: “a445b928e0603d0af0b1c23876adb975c442009aa7c73c1c08bbb9a09bf1918e”) you will find the part of the json related to this transaction.
3.	Copy the value corresponding to "serialized", you can find it in the row just under txid. This is the raw transaction, we will use it in the next steps. It is similar to this:
	```
	020000000001018852b692e876a5ef054eae9466c6096e02b0aa5e0395cd7d2b3421ff90abdd230100000000f2f25c80015b5f03000000000016001490cb3f1d648d41d305d512487a3ef41765a8088104004830450221008e5246ec6dc22b52ee4cfc14de7f6fbf25a19216ccaa7d787b1fa85c0e4fda8502207338d66c692cf7f7987f2901870777fcf4c61c754e6e4fccc89f80ab9b66561501473044022052aaf4d85600198e567cea40dacc248dc700ab9f0ef17235c71a534fb9202a37022045749ae4918ca89c0969a9cf7738f28286ef6c1386fb33105dab11e2b58ab3f50147522102948634a3000bd6b92c757f296928bfc5f26d8fc5eb9bd2d037b0c5e815cf43dd21038a0fbe274e9fbeb9f091c37c008dfc372c51150c4af3b109a72b6d26e644784d52aecce45820
	```
4.	Make sure your bitcoind is running and enter the following command:
    ```
    bitcoin-cli decoderawtransaction <raw transaction>
    ```
    Use the raw transaction we obtained in the previous step.

    Output:
	```
    {
      "txid": "a445b928e0603d0af0b1c23876adb975c442009aa7c73c1c08bbb9a09bf1918e",
      "hash": "622f29bd90255cfb7a5c7e1656a53372043f12e9777d5ce4c6f50cf0276a5c78",
      "version": 2,
      "size": 303,
      "vsize": 138,
      "weight": 549,
      "locktime": 542696652,
      "vin": [
        {
          "txid": "23ddab90ff21342b7dcd95035eaab0026e09c66694ae4e05efa576e892b65288",
          "vout": 1,
          "scriptSig": {
            "asm": "",
            "hex": ""
          },
          "txinwitness": [
            "",
            "30450221008e5246ec6dc22b52ee4cfc14de7f6fbf25a19216ccaa7d787b1fa85c0e4fda8502207338d66c692cf7f7987f2901870777fcf4c61c754e6e4fccc89f80ab9b66561501",
            "3044022052aaf4d85600198e567cea40dacc248dc700ab9f0ef17235c71a534fb9202a37022045749ae4918ca89c0969a9cf7738f28286ef6c1386fb33105dab11e2b58ab3f501",
            "522102948634a3000bd6b92c757f296928bfc5f26d8fc5eb9bd2d037b0c5e815cf43dd21038a0fbe274e9fbeb9f091c37c008dfc372c51150c4af3b109a72b6d26e644784d52ae"
          ],
          "sequence": 2153575154
        }
      ],
      "vout": [
        {
          "value": 0.00221019,
          "n": 0,
          "scriptPubKey": {
            "asm": "0 90cb3f1d648d41d305d512487a3ef41765a80881",
            "hex": "001490cb3f1d648d41d305d512487a3ef41765a80881",
            "reqSigs": 1,
            "type": "witness_v0_keyhash",
            "addresses": [
              "bc1qjr9n78ty34qaxpw4zfy850h5zaj6szyph7n6am"
            ]
          }
        }
      ]
    }
	```
5.	Now looking at the output of `decoderawtransaction`, we see the signature/witness stack, let's operate on that first:
	```
    "txinwitness": [
            "",
            "30450221008e5246ec6dc22b52ee4cfc14de7f6fbf25a19216ccaa7d787b1fa85c0e4fda8502207338d66c692cf7f7987f2901870777fcf4c61c754e6e4fccc89f80ab9b66561501",
            "3044022052aaf4d85600198e567cea40dacc248dc700ab9f0ef17235c71a534fb9202a37022045749ae4918ca89c0969a9cf7738f28286ef6c1386fb33105dab11e2b58ab3f501",
            "522102948634a3000bd6b92c757f296928bfc5f26d8fc5eb9bd2d037b0c5e815cf43dd21038a0fbe274e9fbeb9f091c37c008dfc372c51150c4af3b109a72b6d26e644784d52ae"
          ],
	```
	Line one is the signature for key A, line two is the signature for key B, line three is the witness script.
6.	Using the witness script obtained above (line 3 in "txinwitness", previous step) we are going to obtain the two public keys with the following command:
    ```
    bitcoin-cli decodescript <witness script>
    ```

    Output:
	```
    {
      "asm": "2 02948634a3000bd6b92c757f296928bfc5f26d8fc5eb9bd2d037b0c5e815cf43dd 038a0fbe274e9fbeb9f091c37c008dfc372c51150c4af3b109a72b6d26e644784d 2 OP_CHECKMULTISIG",
      "reqSigs": 2,
      "type": "multisig",
      "addresses": [
        "1DNikLaNtbgoaAJ6ANX4JYFcJKwbxYxgt7",
        "16QKdrCR4TAwp5pM2R4uq3b3Ad114qFJ4g"
      ],
      "p2sh": "3PX158HUtCcLKeQuh3xzNY1rs8TCGQWcx3",
      "segwit": {
        "asm": "0 e2d5738f3a404873001ccce6a4cf0a65fe26ac43f33c3c0eee61e578e9859aa9",
        "hex": "0020e2d5738f3a404873001ccce6a4cf0a65fe26ac43f33c3c0eee61e578e9859aa9",
        "reqSigs": 1,
        "type": "witness_v0_scripthash",
        "addresses": [
          "bc1qut2h8re6gpy8xqquenn2fnc2vhlzdtzr7v7rcrhwv8jh36v9n25sdcl53r"
        ],
        "p2sh-segwit": "3EVhkpRuazcJTKmBg8ZcRvqfNUGRMhWR34"
      }
    }
	```
      Key A and key B are the two strings between the number 2 in the first “asm”.
 
7.	Using PSBT-Toolkit we are going to convert the transaction into a base PSBT by clicking "From Unsigned Transaction" and pasting the raw transaction we got in the step 3. The result has to be similar to this:
	```
  	cHNidP8BAFICAAAAAYhStpLodqXvBU6ulGbGCW4CsKpeA5XNfSs0If+Qq90jAQAAAADy8lyAAVtfAwAAAAAAFgAUkMs/HWSNQdMF1RJIej70F2WoCIHM5FggAAAA”
    ```
    Leave PSBT-Toolkit open.

8.	Now from the output of step 4 we can obtain the initial channel opening transaction ID (can be found in "vin": [    {   "txid"). Then we have to go to this [page](https://nioctib.tech/#/transaction) and insert the initial channel opening transaction ID. In this example: "23ddab90ff21342b7dcd95035eaab0026e09c66694ae4e05efa576e892b65288".
    Here we have to understand what is the output which generated the channel (looking at its amount for example), mind that the first is output zero. Then click on RAW. Going with the mouse above the text give you the meaning of the different colors. So we copy the green and red part of the correct output (In this example: "3063030000000000220020e2d5738f3a404873001ccce6a4cf0a65fe26ac43f33c3c0eee61e578e9859aa9"), we click on "Add Witness UTXO" in the PSBT-Toolkit, enter Input Index 0 and past the copied string in the Output field. Then press OK.

9.	Now click on "Add Input Redeem Script" in the PSBT-Toolkit, again index 0 and paste in the second field the witness script (line three of step 5). Result:
	```
	cHNidP8BAFICAAAAAYhStpLodqXvBU6ulGbGCW4CsKpeA5XNfSs0If+Qq90jAQAAAADy8lyAAVtfAwAAAAAAFgAUkMs/HWSNQdMF1RJIej70F2WoCIHM5FggAAEBKzBjAwAAAAAAIgAg4tVzjzpASHMAHMzmpM8KZf4mrEPzPDwO7mHleOmFmqkBBEdSIQKUhjSjAAvWuSx1fylpKL/F8m2Pxeub0tA3sMXoFc9D3SEDig++J06fvrnwkcN8AI38NyxRFQxK87EJpyttJuZEeE1SrgAA
	```
	Save it somewhere, because now we can follow two different possibilities.

10.	**Variant A:** we assume your key is key A (in this example: "02948634a3000bd6b92c757f296928bfc5f26d8fc5eb9bd2d037b0c5e815cf43dd"). So we click on the "Add Unknown" button in PSBT-Toolkit (middle column under Input Functions), insert key A in the field Data, Input Index 0 and “cc” in the field Key (which is a custom key for `chantools` so it knows what key to look for).
11.	Now we add the public key and the signature of B. So click on “Add signature” in PSBT-Toolkit, insert Input Index 0, Public Key of B (in this example: "038a0fbe274e9fbeb9f091c37c008dfc372c51150c4af3b109a72b6d26e644784d") and signature of B (in this example: "3044022052aaf4d85600198e567cea40dacc248dc700ab9f0ef17235c71a534fb9202a37022045749ae4918ca89c0969a9cf7738f28286ef6c1386fb33105dab11e2b58ab3f501")
12.	PSBT-Toolkit currently has a little bug, we just need to fix the script. It's marked as "redeem script" (0x04) in the PSBT but we actually want a "witness script" (0x05). So we just convert the obtained string from Base64 to Hex, look for the "04" in the string (2 bytes before the actual script starts, so you can just search for the witness script we obtained in line three of step 5) and change it to "05". Then convert everything back to Base64.
	There are a lot of free tools online for string conversion. Otherwise you can use the following simple commands.
	For Base64 to Hex:
	`echo "<base64>" | base64 -d | xxd -p -c999`.
	For Hex to Base64:
	`echo "<hex>" | xxd -p -r | base64 -w0`.

	Now you have the corrected PSBT! In this example:
	```
	cHNidP8BAFICAAAAAYhStpLodqXvBU6ulGbGCW4CsKpeA5XNfSs0If+Qq90jAQAAAADy8lyAAVtfAwAAAAAAFgAUkMs/HWSNQdMF1RJIej70F2WoCIHM5FggAAHMIQKUhjSjAAvWuSx1fylpKL/F8m2Pxeub0tA3sMXoFc9D3QEBKzBjAwAAAAAAIgAg4tVzjzpASHMAHMzmpM8KZf4mrEPzPDwO7mHleOmFmqkiAgOKD74nTp++ufCRw3wAjfw3LFEVDErzsQmnK20m5kR4TUcwRAIgUqr02FYAGY5WfOpA2swkjccAq58O8XI1xxpTT7kgKjcCIEV0muSRjKicCWmpz3c48oKG72wThvszEF2rEeK1irP1AQEFR1IhApSGNKMAC9a5LHV/KWkov8XybY/F65vS0DewxegVz0PdIQOKD74nTp++ufCRw3wAjfw3LFEVDErzsQmnK20m5kR4TVKuAAA=)
	```
13.	Run the following command inserting the obtained PSBT:
	```
	chantools signrescuefunding --psbt <your-PSBT>
	```

14.	If last command succeeds simply send the raw transaction you obtained using bitcoind (`bitcoin-cli sendrawtransaction <raw-transaction>`). If the command in step 13 doesn’t succeed and you get "could not find local multisig key" then we need to try with **Variant B: insert in PSBT Toolkit the string you saved in step 9 and repeat the procedure from step 10 assuming your key is Key B and replacing Key A and Signature A with Key B and Signature B and vice versa.**
