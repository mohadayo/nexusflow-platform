import { app } from "./app";

const PORT = parseInt(process.env.DASHBOARD_PORT || "3000", 10);

app.listen(PORT, "0.0.0.0", () => {
  console.log(
    `${new Date().toISOString()} [INFO] dashboard: Starting on port ${PORT}`
  );
});
