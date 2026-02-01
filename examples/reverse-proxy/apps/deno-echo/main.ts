export default {
  fetch(req: Request) {
    const url = new URL(req.url);
    let headersText = "";
    for (const [key, value] of req.headers.entries()) {
      headersText += `${key}: ${value}\n`;
    }

    const response = `Request Headers:\n${headersText}\nLocation: ${url.pathname}`;
    return new Response(response, {
      status: 200,
      headers: { "Content-Type": "text/plain" },
    });
  },
};
