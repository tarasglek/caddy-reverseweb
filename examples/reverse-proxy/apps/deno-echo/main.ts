export default {
  fetch(req: Request) {
    const url = new URL(req.url);
    let headersText = "";
    for (const [key, value] of req.headers.entries()) {
      headersText += `${key}: ${value}\n`;
    }

    let envText = "";
    for (const [key, value] of Object.entries(Deno.env.toObject())) {
      envText += `${key}=${value}\n`;
    }

    const response = `Request Headers:\n${headersText}\nEnvironment Variables:\n${envText}\nLocation: ${url.pathname}`;
    return new Response(response, {
      status: 200,
      headers: { "Content-Type": "text/plain" },
    });
  },
};
