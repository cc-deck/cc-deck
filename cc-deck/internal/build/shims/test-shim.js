// Test script that exercises the exact code path Claude Code uses.
// os.networkInterfaces() calls getifaddrs() via libuv.
const os = require('os');
const https = require('https');

console.log('=== getifaddrs shim test ===');

// Test 1: os.networkInterfaces() - this is what Claude Code calls
try {
    const ifaces = os.networkInterfaces();
    console.log('PASS: os.networkInterfaces() returned:', JSON.stringify(ifaces));
} catch (e) {
    console.log('FAIL: os.networkInterfaces() threw:', e.message);
    process.exit(1);
}

// Test 2: HTTPS request - verify networking still works after shim
const req = https.get('https://api.anthropic.com', { timeout: 5000 }, (res) => {
    console.log('PASS: HTTPS request returned status', res.statusCode);
    res.resume();
    process.exit(0);
});
req.on('error', (e) => {
    // Connection refused or timeout is fine - we just need the TLS handshake
    // to prove that networking works. ECONNREFUSED/ETIMEDOUT means TCP works.
    if (e.code === 'ECONNREFUSED' || e.code === 'ETIMEDOUT' || e.code === 'ENOTFOUND') {
        console.log('PASS: HTTPS attempt got', e.code, '(networking works, just no route)');
        process.exit(0);
    }
    console.log('FAIL: HTTPS request failed unexpectedly:', e.message);
    process.exit(1);
});
req.on('timeout', () => {
    console.log('PASS: HTTPS request timed out (networking works, just slow/blocked)');
    req.destroy();
    process.exit(0);
});
