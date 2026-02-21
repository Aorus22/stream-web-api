// Node.js wrapper template for custom provider scripts
// This template is used by the Go service to wrap user code
// ARG_QUERY is passed as the first command-line argument

const ARG_QUERY = process.argv[2];

// === USER CODE INJECTED HERE ===
// {{USER_CODE}}

// === OUTPUT HANDLER ===
(async () => {
  try {
    let result = userCode;
    if (typeof result === 'function') {
      result = await result(ARG_QUERY);
    } else if (result && typeof result.then === 'function') {
      result = await result;
    }
    process.stdout.write(JSON.stringify({ result }));
    process.exit(0);
  } catch (error) {
    process.stdout.write(JSON.stringify({
      result: null,
      error: error.message || String(error)
    }));
    process.exit(0);
  }
})();
