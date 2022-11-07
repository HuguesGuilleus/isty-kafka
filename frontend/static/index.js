(function() {
	const inputSumbit = document.querySelector("input[type=submit]"),
		inputText = document.querySelector("input[type=text]"),
		outputZone = document.querySelector("#outputZone");

	inputSumbit.addEventListener("click", _ => {
		inputText.value &&
			fetch("message-send", {
				method: "PUT",
				body: inputText.value
			});
		inputText.value = "";
	});

	function addElement(message) {
		const p = document.createElement("p");
		p.innerText = message;
		outputZone.append(p);
	}
	const eventSource = new EventSource("message-get");
	eventSource.onmessage = event => addElement(event.data);
	eventSource.onclose = _ => addElement("[CONNEXION FERMÉE]");
})();
