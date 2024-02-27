import requests
from biscepter_api_client import Client
from biscepter_api_client.models import OffendingCommit, RunningSystem
from biscepter_api_client.types import Response
from biscepter_api_client.api.default import get_system, post_is_bad_system_id, post_is_good_system_id

# Init the API client
client = Client(base_url="http://localhost:40032")

with client as client:
    while True:
        # Get the next running system or offending commit
        res = get_system.sync(client=client)

        if isinstance(res, OffendingCommit):
            # We got an offending commit
            print("Bisection done! Offending commit", res.commit)
            print("Commit message:", res.commit_message)
            exit(0)
        elif isinstance(res, RunningSystem):
            # We got a running system
            print("Got running system on port", res.ports["3333"])

            # Send the request to the url /1
            url = "http://localhost:" + res.ports["3333"] + "/1"
            response = requests.get(url)
            print("Got response", response.text)

            if response.text == "1":
                # Response is what we expected -> Commit is good
                print("This commit is good!")
                post_is_good_system_id.sync_detailed(res.system_index, client=client)
            else:
                # Response is not what we expected -> Commit is bad
                print("This commit is bad!")
                post_is_bad_system_id.sync_detailed(res.system_index, client=client)
