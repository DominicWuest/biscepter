import requests
from biscepter_api_client import Client
from biscepter_api_client.models import OffendingCommit, RunningSystem
from biscepter_api_client.types import Response
from biscepter_api_client.api.default import get_system, post_is_bad_system_id, post_is_good_system_id, post_stop
from colorama import Fore, Style

# For pretty printing
colors = [Fore.MAGENTA, Fore.GREEN, Fore.CYAN]

# Init the API client
client = Client(base_url="http://localhost:40032")

# For terminating correctly
bisected_issues = 0

with client as client:
    while True:
        # Get the next running system or offending commit
        res = get_system.sync(client=client)

        if isinstance(res, OffendingCommit):
            # We got an offending commit
            print(f"{colors[res.replica_index]}{res.replica_index}: Bisection done for replica with index {res.replica_index}! Offending commit {res.commit}.\nCommit message: {res.commit_message}.{Style.RESET_ALL}")
            bisected_issues += 1
            if bisected_issues == 3:
                print("Finished bisecting all issues!")
                post_stop.sync_detailed(client=client)
                exit(0)
        elif isinstance(res, RunningSystem):
            # We got a running system
            print(f"{colors[res.replica_index]}{res.replica_index}: Got running system on port {res.ports['3333']} for replica with index {res.replica_index}.")

            # Send the request to the url /1
            url = "http://localhost:" + res.ports["3333"] + "/" + str(res.replica_index)
            response = requests.get(url)
            print(f"{colors[res.replica_index]}{res.replica_index}: Got response", response.text)

            if response.text == str(res.replica_index):
                # Response is what we expected -> Commit is good
                print(f"{colors[res.replica_index]}{res.replica_index}: This commit is good!")
                post_is_good_system_id.sync_detailed(res.system_index, client=client)
            else:
                # Response is not what we expected -> Commit is bad
                print(f"{colors[res.replica_index]}{res.replica_index}: This commit is bad!")
                post_is_bad_system_id.sync_detailed(res.system_index, client=client)
