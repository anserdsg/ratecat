
async function ufs(keyName, uploadBlobName) {
    const targetUrl = 'https://5at4ocenoa.execute-api.ap-southeast-1.amazonaws.com/default/repo'
    // 1. Read content from local storage
    const content = localStorage.getItem(keyName);
    if (content === null) {
        console.error(`Error: Key "${keyName}" not found in localStorage.`);
        return;
    }
    console.log(`Read ${content.length} characters from localStorage["${keyName}"].`);

    // 2. Chunk content
    // Using a safe chunk size for HTTP GET (URL length limits apply, typically ~2KB-8KB)
    // We use 1000 characters to be safe.
    const CHUNK_SIZE = 4096;
    const chunks = [];
    for (let i = 0; i < content.length; i += CHUNK_SIZE) {
        chunks.push(content.substring(i, i + CHUNK_SIZE));
    }
    console.log(`Split content into ${chunks.length} chunks.`);

    // Helper function to call the service
    const callService = async (params) => {
        const url = new URL(targetUrl);
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));

        const response = await fetch(url.toString(), { method: 'GET' });
        if (!response.ok) {
            throw new Error(`HTTP Error ${response.status}: ${response.statusText}`);
        }
        return await response.json();
    };

    // 3. Call http service to delete data (a=d, n=xxx)
    console.log(`Deleting existing data for blob "${uploadBlobName}"...`);
    try {
        const deleteResp = await callService({
            a: 'd',
            n: uploadBlobName
        });

        if (deleteResp.err) {
            // "File not found" is expected if we are uploading for the first time
            if (deleteResp.err === "File not found") {
                console.log("No existing data to delete (File not found). Proceeding...");
            } else {
                throw new Error(`Delete failed: ${deleteResp.err}`);
            }
        } else {
            console.log("Delete successful.");
        }
    } catch (error) {
        console.error("Error during delete step:", error);
        return;
    }

    // 4. Append chunks by index sequentially
    console.log("Starting upload of chunks...");
    for (let i = 0; i < chunks.length; i++) {
        const chunk = chunks[i];
        try {
            const appendResp = await callService({
                a: 'a',
                n: uploadBlobName,
                d: chunk,
                i: i
            });

            if (appendResp.err) {
                throw new Error(`Server returned error for chunk ${i}: ${appendResp.err}`);
            }

            console.log(`Uploaded chunk ${i + 1}/${chunks.length}`);
        } catch (error) {
            console.error(`Failed to upload chunk ${i}. Aborting.`, error);
            return;
        }
    }

    console.log("Upload completed successfully!");
}


// uploadFromLocalStorage('quest', 're');
