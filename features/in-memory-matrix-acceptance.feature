Feature: In-memory Matrix acceptance backend

  # The fast acceptance backend is a test fixture, not a production transport.
  # It still runs the real bridge process, the forge dashboard and the same
  # Gherkin steps. Real mautrix clients keep their Olm keys and encrypt events;
  # the in-process server only relays the opaque ciphertext and room metadata.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the Matrix fixture backend is memory
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started

  Scenario Outline: The in-memory backend carries an encrypted request and its threaded answer
    Given the forge's dashboard already holds the chat request "<request>"
    Then the operator decrypts the chat message "<expected_request>"
    And the bridge sent the chat message "<expected_request>" encrypted
    When the lieutenant answers the chat request "<request>" with "<answer>"
    Then the operator decrypts the thread reply "<expected_answer>" to the chat message "<expected_request>"

    Examples:
      | request             | expected_request    | answer                   | expected_answer          |
      | is the build green? | is the build green? | Yes, the build is green. | Yes, the build is green. |
      | is the deploy green? | is the deploy green? | Yes, the deploy is green. | Yes, the deploy is green. |
