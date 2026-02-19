#!/usr/bin/env bash
# =============================================================================
# Generate self-signed TLS certificates for local development.
#
# Creates a local CA and a server certificate signed by that CA.
# Certificates are placed in the certs/ directory at the project root.
#
# Usage:
#   ./scripts/generate-dev-certs.sh
#
# The script is idempotent: it skips generation if certificates already exist.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CERTS_DIR="${PROJECT_ROOT}/certs"

CA_KEY="${CERTS_DIR}/ca.key"
CA_CERT="${CERTS_DIR}/ca.crt"
SERVER_KEY="${CERTS_DIR}/server.key"
SERVER_CERT="${CERTS_DIR}/server.crt"
SERVER_CSR="${CERTS_DIR}/server.csr"

DAYS_VALID=365

# Check if certificates already exist.
if [[ -f "${CA_CERT}" && -f "${CA_KEY}" && -f "${SERVER_CERT}" && -f "${SERVER_KEY}" ]]; then
    echo "Certificates already exist in ${CERTS_DIR}. Skipping generation."
    echo "  CA:     ${CA_CERT}"
    echo "  Server: ${SERVER_CERT}"
    echo "To regenerate, remove the certs/ directory first."
    exit 0
fi

echo "Generating development TLS certificates..."
mkdir -p "${CERTS_DIR}"

# ---------------------------------------------------------------------------
# Step 1: Generate CA private key and self-signed certificate
# ---------------------------------------------------------------------------
echo "  [1/3] Creating Certificate Authority..."
openssl genrsa -out "${CA_KEY}" 4096 2>/dev/null
openssl req -new -x509 \
    -key "${CA_KEY}" \
    -out "${CA_CERT}" \
    -days "${DAYS_VALID}" \
    -subj "/C=US/ST=Dev/L=Local/O=smtp-proxy/OU=Development/CN=smtp-proxy-dev-ca" \
    2>/dev/null

# ---------------------------------------------------------------------------
# Step 2: Generate server private key and CSR
# ---------------------------------------------------------------------------
echo "  [2/3] Creating server certificate..."
openssl genrsa -out "${SERVER_KEY}" 2048 2>/dev/null

# Create a temporary OpenSSL config with SAN entries.
OPENSSL_CNF=$(mktemp)
trap 'rm -f "${OPENSSL_CNF}"' EXIT

cat > "${OPENSSL_CNF}" <<EOF
[req]
default_bits       = 2048
prompt             = no
distinguished_name = dn
req_extensions     = v3_req

[dn]
C  = US
ST = Dev
L  = Local
O  = smtp-proxy
OU = Development
CN = localhost

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = smtp-server
DNS.3 = api-server
IP.1  = 127.0.0.1
EOF

openssl req -new \
    -key "${SERVER_KEY}" \
    -out "${SERVER_CSR}" \
    -config "${OPENSSL_CNF}" \
    2>/dev/null

# ---------------------------------------------------------------------------
# Step 3: Sign the server certificate with the CA
# ---------------------------------------------------------------------------
echo "  [3/3] Signing server certificate with CA..."
openssl x509 -req \
    -in "${SERVER_CSR}" \
    -CA "${CA_CERT}" \
    -CAkey "${CA_KEY}" \
    -CAcreateserial \
    -out "${SERVER_CERT}" \
    -days "${DAYS_VALID}" \
    -extfile "${OPENSSL_CNF}" \
    -extensions v3_req \
    2>/dev/null

# Clean up CSR and serial file.
rm -f "${SERVER_CSR}" "${CERTS_DIR}/ca.srl"

echo ""
echo "Development certificates generated successfully:"
echo "  CA certificate:     ${CA_CERT}"
echo "  CA private key:     ${CA_KEY}"
echo "  Server certificate: ${SERVER_CERT}"
echo "  Server private key: ${SERVER_KEY}"
echo ""
echo "SANs: DNS:localhost, DNS:smtp-server, DNS:api-server, IP:127.0.0.1"
echo "Valid for: ${DAYS_VALID} days"
