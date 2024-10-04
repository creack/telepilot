# Inputs:
# - MTLS_CLIENTS: List of client certificates to generate.

CFSSL_IMG = cfssl
CFSSL_BIN = docker run --rm -i -u "$(shell id -u):$(shell id -g)" -v "${PWD}:/ssl" -w /ssl ${CFSSL_IMG}

.PHONY: mtls
mtls: certs/server.pem ${MTLS_CLIENTS:%=certs/client-%.pem}

# Generate the self-signed root CA.
certs/ca-key.pem certs/ca.pem: .build/docker-${CFSSL_IMG} make/csr.json
	@mkdir -p $(dir $@)
	sed 's/{{CN}}/ca/' make/csr.json | ${CFSSL_BIN} cfssl genkey -initca - | ${CFSSL_BIN} sh -c 'cd certs && cfssljson -bare ca'
	@rm certs/ca.csr

# Generate the Private key and Cert Sign Requests (CSR).
certs/%.csr certs/%-key.pem: .build/docker-${CFSSL_IMG} make/csr.json
	@mkdir -p $(dir $@)
	cat make/csr.json | sed 's/{{CN}}/${*}/' | ${CFSSL_BIN} cfssl genkey - | ${CFSSL_BIN} sh -c 'cd certs && cfssljson -bare ${*}'

# Generate the Certificates.
certs/%.pem: certs/%.csr certs/%-key.pem certs/ca.pem certs/ca-key.pem make/csr.json make/cfssl.json
	cat make/csr.json | sed 's/{{CN}}/${*}/' | ${CFSSL_BIN} sh -c 'cd certs && cfssl sign -ca ca.pem -ca-key ca-key.pem -config ../make/cfssl.json -profile ${*} ${*}.csr' | ${CFSSL_BIN} sh -c 'cd certs && cfssljson -bare ${*}'

# Cleanup.
.PHONY: clean-mts
clean-mtls:
	rm -rf certs
clean: clean-mtls
