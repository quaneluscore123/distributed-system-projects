package server

import (
	"html/template"
	"net/http"
)

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {

	tmpl := template.Must(template.New("dashboard").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>RoseDB Dashboard</title>

<style>
body{
	font-family:Arial,sans-serif;
	background:#f5f5f5;
	padding:30px;
}

.card{
	background:white;
	padding:20px;
	border-radius:10px;
	max-width:600px;
	box-shadow:0 2px 8px rgba(0,0,0,.1);
}

h1{
	color:#333;
}

.item{
	margin:10px 0;
	font-size:18px;
}
</style>

</head>

<body>

<div class="card">

<h1>RoseDB Dashboard</h1>

<div class="item">
Status:
<span id="status">Loading...</span>
</div>

<div class="item">
Role:
<span id="role">Loading...</span>
</div>

<div class="item">
Uptime:
<span id="uptime">Loading...</span>
</div>

<div class="item">
Memory:
<span id="memory">Loading...</span>
</div>

</div>

<script>

async function loadHealth(){

	try{

		const res = await fetch('/health');
		const data = await res.json();

		document.getElementById('status').innerText =
			data.status;

		document.getElementById('role').innerText =
			data.role;

		document.getElementById('uptime').innerText =
			data.uptime;

		document.getElementById('memory').innerText =
			data.memory_alloc_mb.toFixed(2) + " MB";

	}catch(err){

		document.getElementById('status').innerText =
			"OFFLINE";

	}

}

loadHealth();

setInterval(loadHealth,2000);

</script>

</body>
</html>
`))

	tmpl.Execute(w, nil)
}
