import React from "react";
import { Typography, Box, Divider } from "@mui/material";

import HttpHeadersTable from "./HttpHeadersTable";
import Editor from "./Editor";

interface Props {
  request: {
    method: string;
    url: string;
    proto: string;
    headers: Array<{ key: string; value: string }>;
    body?: string;
  };
}

function RequestDetail({ request }: Props): JSX.Element {
  const { method, url, proto, headers, body } = request;

  const contentType = headers.find((header) => header.key === "Content-Type")?.value;
  const parsedUrl = new URL(url);

  return (
    <div>
      <Box p={2}>
        <Typography variant="overline" color="textSecondary" style={{ float: "right" }}>
          Request
        </Typography>
        <Typography
          sx={{
            width: "calc(100% - 80px)",
            fontSize: "1rem",
            wordBreak: "break-all",
            whiteSpace: "pre-wrap",
          }}
          variant="h6"
        >
          {method} {decodeURIComponent(parsedUrl.pathname + parsedUrl.search)}{" "}
          <Typography component="span" color="textSecondary" style={{ fontFamily: "'JetBrains Mono', monospace" }}>
            {proto}
          </Typography>
        </Typography>
      </Box>

      <Divider />

      <Box p={2}>
        <HttpHeadersTable headers={headers} />
      </Box>

      {body && <Editor content={body} contentType={contentType} />}
    </div>
  );
}

export default RequestDetail;
